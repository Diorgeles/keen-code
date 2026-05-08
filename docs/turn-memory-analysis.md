# Turn Memory: KV Cache Impact Analysis by Claude Opus 4.6

## How Turn Memory Works

1. During an assistant turn, `turnMemoryAccumulator` records:
   - Files changed (via `write_file`/`edit_file` tool calls)
   - Failed bash commands (non-zero exit codes)

2. When the turn ends (`handleLLMDone`), it attaches this `TurnMemory` to the assistant `Message` struct.

3. **The actual tool call details (inputs, outputs) are NOT stored in the message history.** Only the assistant's text response + the compact turn memory summary is kept.

4. On the next request, `FormatMessageForProvider` appends the turn memory to the assistant message content:
   ```
   <assistant's text response>

   Tool memory:
   - Files changed: foo.go, bar.go
   - Failed bash: go test ./... (exit 1)
   ```

5. This is what the provider sees — **not** the full tool call/response pairs.

## KV Cache Impact: Significant, but a deliberate tradeoff

The core issue is that **between turns within a single agentic loop, the provider sees the full tool calls** (they're happening in real-time via the streaming protocol). But when the turn *ends* and a new user message triggers the *next* request, those tool calls are **gone** — replaced by the compact turn memory summary.

This means the message history sent on **turn N+1** does not match what was sent on **turn N**:

```
Turn N (streaming, provider sees):
  [system] [user1] [assistant text + tool_use] [tool_result] [assistant text + tool_use] [tool_result] [assistant final]

Turn N+1 (new request):
  [system] [user1] [assistant: "final text\n\nTool memory:\n- Files changed: foo.go"]  [user2]
                    ^^^^ DIFFERENT from what was cached ^^^^
```

**The assistant message for turn N has changed content between requests.** The KV cache hit stops right before that assistant message — everything after it (including all subsequent messages) is a cache miss.

## Quantifying the damage

| Scenario | Cache hit prefix | Cache miss |
|----------|-----------------|------------|
| Normal multi-turn (no tools used) | All messages except the new user msg | Just the new user msg |
| After a tool-heavy turn | System + messages up to the **modified** assistant msg | Everything from that assistant msg onward |

In practice:
- **Turn 1**: Cold start, full miss (unavoidable)
- **Turn 2 (user replies)**: Cache hits system prompt. Misses on the compressed assistant message from turn 1 (it's different from what was streamed). Misses on new user message. **Cost: re-process the compressed assistant msg + new user msg.**
- **Turn 3**: Cache hits system + user1 + compressed-assistant1 + user2. Misses on compressed-assistant2 + new user msg.

After the *first* re-request following a tool turn, that compressed assistant message becomes the **new stable representation** and is cached going forward. The one-time cost per turn transition is re-processing a single assistant message (which is much smaller than the original tool calls would have been).

## The tradeoff is correct

**Without turn memory (keeping full tool calls):**
- Perfect KV cache continuity between turns
- But context window fills up 5-10x faster (tool outputs can be huge: file contents, grep results, bash output)
- Hits context limit sooner → forces compaction or truncation → **massive** cache bust

**With turn memory (current approach):**
- One-time cache miss per assistant message on the next turn
- But context stays small → more turns before compaction → fewer total cache busts over the session lifetime

**Net effect:** Turn memory **helps** KV cache efficiency overall because it keeps the total token count low, which means:
1. Fewer compaction events (each one is a complete cache reset)
2. More of the conversation fits in the cache window
3. The per-turn miss is tiny (a few hundred tokens of compressed summary vs. thousands of tokens of tool output)

## Token Cost: Unambiguously Cheaper

**Yes, significantly fewer tokens.** The KV cache concern is negligible compared to the raw token savings.

### The math

Assume a tool-heavy turn produces ~15k tokens of tool interactions (a few `read_file` + `grep` + `bash` calls). Turn memory compresses that to ~100-200 tokens.

After 10 tool-heavy turns:

| | Without turn memory | With turn memory |
|---|---|---|
| Context sent on turn 11 | ~150k tokens | ~3k tokens |
| Cached portion (cheap) | ~135k | ~2.7k |
| Uncached portion (full price) | ~15k | ~300 |

Even cached input tokens aren't free (Anthropic charges 0.1x, OpenAI has limits on cache duration). Sending 150k cached tokens still costs more than sending 3k tokens with a small cache miss.

### Where the real savings compound

1. **Every subsequent turn re-sends the full history.** A 15k tool output from turn 2 gets re-sent on turns 3, 4, 5, ... forever. Even at cached rates, that accumulates.

2. **Compaction avoidance.** Without turn memory, you hit context limits 10-50x sooner. Compaction means:
   - Output tokens to generate the summary (expensive)
   - Complete KV cache reset (everything re-processed at full price)
   - Information loss

3. **Output tokens are the most expensive.** By keeping context small, the model can focus — less "needle in a haystack" → shorter, more direct responses.

### The KV cache miss cost in perspective

The one-time miss per turn boundary: re-process ~200 tokens of compressed summary. That's ~$0.0006 on Anthropic.

The savings from not carrying 15k tokens of tool output through every subsequent turn: orders of magnitude more.

## Summary

Turn memory causes a one-time cache miss per assistant turn boundary (the compressed summary differs from the streamed tool calls). But this is the right tradeoff — it keeps context small, delays compaction, and avoids much larger cache busts down the line. The per-turn cost is a few hundred tokens of re-processing; the savings are avoiding re-processing the entire conversation when context overflows.

Turn memory is unambiguously cheaper. The cache miss is a rounding error compared to the token volume reduction.
