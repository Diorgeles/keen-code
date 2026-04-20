# Turn Memory vs KV Cache

As of 2026-04-20.

## Question

Keen Code keeps raw tool calls and tool outputs only within a single model turn, then collapses the turn into:

- the assistant's final text reply
- a compact `TurnMemory` block that currently records:
  - changed files
  - failed bash commands

The question is whether dropping the raw tool transcript harms provider KV cache / prompt cache behavior enough to outweigh the token savings.

## Short Answer

Usually no.

Across turns, the current `TurnMemory` design is generally cheaper than retaining full historical tool transcripts, even on providers that discount cached input tokens:

- cached tokens are discounted, not free
- smaller retained history means fewer cache writes, fewer cache reads, and lower prefill work
- the main downside is not cache economics, but recoverability:
  - if the model later needs details from an old tool output, it may have to rerun tools

So the trade is:

- `TurnMemory`: lower steady-state token cost, higher chance of retooling later
- full raw tool transcript retention: higher steady-state token cost, lower chance of retooling later

## What Keen Code Does Today

### Cross-turn state

Conversation reconstruction only keeps assistant message text plus `TurnMemory`:

- `internal/session/projection.go:17-23`
- `internal/llm/message_format.go:8-27`
- `internal/llm/systemprompt.go:52-57`

Implication:

- old raw tool requests and raw tool results are not part of future-turn prompts
- future turns therefore do not pay input-token costs for those old transcripts

### Within-turn state

Each provider client does retain tool traffic while the same turn is still running:

- OpenAI-compatible Chat Completions appends assistant tool calls and tool result messages within the same loop:
  - `internal/llm/openai.go:279-314`
- OpenAI Responses keeps `previous_response_id` only inside the current turn loop:
  - `internal/llm/openai_responses.go:127-143`
- Anthropic appends assistant blocks and tool-result blocks within the same loop:
  - `internal/llm/anthropic.go:243-279`
- Genkit appends model tool requests and tool responses within the same loop:
  - `internal/llm/genkit.go:101-156`

Implication:

- same-turn tool chaining benefits from the provider's native incremental context handling
- cross-turn history is where the raw transcript is dropped

## Why This Matters For Cache Economics

Provider prompt caching only helps when a later request repeats the same prefix, or a sufficiently similar prefix for providers with implicit caching.

If Keen Code kept old tool transcripts forever:

- the prompt would be larger
- more of the old prompt could become cacheable
- but cached reads still cost money
- initial cache writes also cost money on providers with explicit prompt caching

If Keen Code keeps only compact `TurnMemory`:

- fewer tokens are sent on every future turn
- there is less material available to cache, but there is also less material to pay for

That means the cost question is not "does caching get worse?"

It is:

"Is the discounted cost of keeping old raw tool transcripts lower than the expected cost of rerunning tools when details are needed again?"

In most agent sessions, the answer is no.

## Provider-by-Provider Impact

### OpenAI

#### Cache behavior

OpenAI prompt caching:

- is automatic on recent models
- only works for exact prefix matches
- starts at 1024 input tokens
- treats tools as part of the prompt prefix, so identical tool definitions help cache hits

OpenAI docs:

- automatic caching and exact-prefix rule:
  - <https://developers.openai.com/api/docs/guides/prompt-caching>
- pricing:
  - <https://openai.com/api/pricing/>
- GPT-5.3-Codex pricing:
  - <https://developers.openai.com/api/docs/models/gpt-5.3-codex>

Relevant facts:

- GPT-5.4:
  - input: $2.50 / 1M
  - cached input: $0.25 / 1M
  - output: $15.00 / 1M
- GPT-5.4 mini:
  - input: $0.75 / 1M
  - cached input: $0.075 / 1M
  - output: $4.50 / 1M
- GPT-5.3-Codex:
  - input: $1.75 / 1M
  - cached input: $0.175 / 1M
  - output: $14.00 / 1M

#### Effect of `TurnMemory`

Dropping old raw tool transcripts across turns usually still saves money:

- cached reads are 10% of base on the GPT-5.4 family, not 0%
- if a 50k-token tool transcript is retained, GPT-5.4 still costs about:
  - $0.0125 per future turn on cache hit reads
  - $0.125 per future turn on cache misses
- if that transcript is summarized to a ~600-token memory block, the future-turn input cost is drastically smaller even without cache

Important nuance:

- `previous_response_id` in the Responses client helps only inside one `StreamChat` run
- it is reset for each outer assistant turn at `internal/llm/openai_responses.go:129`

So OpenAI still benefits from same-turn context continuity, but cross-turn retention depends on the rebuilt conversation history, which currently uses `TurnMemory`.

#### Conclusion

For OpenAI, `TurnMemory` is usually cheaper than keeping full old tool transcripts.

The cost argument for retaining full transcripts only becomes favorable when:

- old tool outputs are frequently reused verbatim
- rerunning the tools would be expensive or impossible

### Anthropic

#### Cache behavior

Anthropic prompt caching:

- can be enabled with top-level `cache_control`
- also supports explicit per-block breakpoints
- uses cache writes and cache reads with separate pricing
- has model-specific minimum cacheable prompt lengths

Anthropic docs:

- pricing:
  - <https://platform.claude.com/docs/en/about-claude/pricing>
- prompt caching:
  - <https://platform.claude.com/docs/en/build-with-claude/prompt-caching>

Current Keen Code state:

- top-level Anthropic `cache_control` is now set in `internal/llm/anthropic.go:244-250`

Relevant prices:

- Claude Opus 4.6:
  - base input: $5.00 / MTok
  - 5m cache write: $6.25 / MTok
  - cache hit/read: $0.50 / MTok
  - output: $25.00 / MTok
- Claude Sonnet 4.6:
  - base input: $3.00 / MTok
  - 5m cache write: $3.75 / MTok
  - cache hit/read: $0.30 / MTok
  - output: $15.00 / MTok
- Claude Haiku 4.5:
  - base input: $1.00 / MTok
  - 5m cache write: $1.25 / MTok
  - cache hit/read: $0.10 / MTok
  - output: $5.00 / MTok

Minimum cacheable prompt length:

- Opus 4.6: 4096 tokens
- Sonnet 4.6: 2048 tokens
- Haiku 4.5: 4096 tokens

#### Effect of `TurnMemory`

Anthropic's caching changes the economics less than it may seem:

- retaining more transcript means paying more cache writes
- and then still paying non-zero cache reads
- summarizing away historical tool output reduces both costs

Example on Sonnet 4.6:

- retaining an extra 50k tokens of historical tool transcript costs about:
  - first 5m cache write: 50,000 * $3.75 / 1,000,000 = $0.1875
  - each later cache-hit read: 50,000 * $0.30 / 1,000,000 = $0.015

If the same information can be compressed to ~600 tokens in the assistant reply plus `TurnMemory`, future turns are still much cheaper.

#### Conclusion

Even with Anthropic prompt caching enabled, `TurnMemory` is usually the better default.

Anthropic caching improves repeated-prefix economics, but it does not make large historical tool transcripts free.

### Gemini

#### Cache behavior

Gemini has two modes:

- implicit caching:
  - automatic on Gemini 2.5 and newer
  - no cost saving guarantee
- explicit caching:
  - manual
  - cost saving guarantee
  - cached tokens billed at reduced rates plus storage

Gemini docs:

- caching behavior:
  - <https://ai.google.dev/gemini-api/docs/caching>
- pricing:
  - <https://ai.google.dev/gemini-api/docs/pricing>

Current Keen Code state:

- the Google path uses Genkit and does not currently create explicit cached content
- see `internal/llm/genkit.go:108-117`

Relevant supported-model facts:

- Gemini 3 Flash Preview:
  - implicit cache minimum: 1024 tokens
  - input: $0.50 / 1M
  - explicit cached input: $0.05 / 1M
  - explicit cache storage: $1.00 / 1,000,000 tokens per hour
  - output: $3.00 / 1M
- Gemini 3 Pro Preview:
  - implicit cache minimum: 4096 tokens
  - input: $2.00 / 1M
  - explicit cached input: not shown as a simple standalone line for the supported preview here, but explicit caching is a paid reduced-rate feature with storage charges
  - output: $12.00 / 1M

#### Effect of `TurnMemory`

Because Keen Code currently relies only on implicit caching for Gemini:

- cache savings are opportunistic, not guaranteed
- keeping large old tool transcripts is even less attractive than on OpenAI or Anthropic

If explicit caching were added later, the economics would still usually favor summarization because Gemini also adds storage pricing to explicit cache usage.

#### Conclusion

For Gemini, the case for compact `TurnMemory` is strong.

Without explicit caching, large retained transcripts do not reliably convert into lower costs. With explicit caching, storage charges make indiscriminate retention even less attractive.

### DeepSeek

DeepSeek states:

- context caching is enabled by default
- overlapping prefixes produce cache hits
- `deepseek-chat` and `deepseek-reasoner` currently share:
  - input cache hit: $0.028 / 1M
  - input cache miss: $0.28 / 1M
  - output: $0.42 / 1M

Docs:

- <https://api-docs.deepseek.com/guides/kv_cache>
- <https://api-docs.deepseek.com/quick_start/pricing>

Effect:

- retained transcripts are cheap on cache hits compared with other providers
- but they are still 10x more expensive on misses
- compact `TurnMemory` still lowers worst-case cost and prompt size

### Moonshot / Kimi

Moonshot docs confirm:

- automatic context caching is supported
- cached tokens are billed at the cache-hit input rate

Docs:

- <https://platform.kimi.ai/docs/pricing/chat-k2>

The public page render inspected here did not expose a clean machine-readable full price table for all supported Kimi variants, so the safe conclusion is qualitative:

- cache-hit pricing exists
- the same general tradeoff applies
- smaller cross-turn prompts remain cheaper unless old raw tool output is frequently reused

## Why `TurnMemory` Usually Wins

`TurnMemory` wins by default because it reduces:

- future prefill tokens
- future cache writes
- future cache reads
- cache sensitivity to exact-prefix churn

It also avoids dragging long volatile tool outputs through every later turn.

This matters because tool outputs are often:

- bulky
- highly specific
- unlikely to be needed verbatim later
- likely to break exact-prefix matching when anything changes around them

## Where `TurnMemory` Loses

`TurnMemory` can lose when a previous tool output is:

- expensive to reproduce
- not deterministic
- not available anymore
- likely to be referenced directly in later turns

Examples:

- a large search result set that was costly to obtain
- a remote API response that changes quickly
- a long structured analysis the user expects the agent to build on precisely

In those cases, there is a real argument for preserving more than today's compact memory.

## Best Practical Policy

The strongest default policy for Keen Code is:

1. Keep the current compact cross-turn `TurnMemory` model.
2. Let providers exploit same-turn context reuse naturally inside the active tool loop.
3. Use provider caching where it is low-friction and beneficial:
   - Anthropic: enabled now via top-level `cache_control`
   - OpenAI: automatic already
   - Gemini: implicit already; explicit only if there is a clear repeated-large-prefix workload
4. If richer persistence is needed later, add selective retention rather than full transcript retention.

Selective retention is the right next step if needed:

- persist only high-value tool results
- store them in a compact structured form
- avoid replaying every raw tool call and every raw output verbatim

## Recommendation

Keep `TurnMemory` as the default cross-turn mechanism.

Do not retain all raw historical tool calls and outputs by default.

If future work is needed, the best extension is:

- "promote important tool results into durable summary memory"

not:

- "persist the full raw tool transcript forever"

## Sources

- OpenAI prompt caching:
  - <https://developers.openai.com/api/docs/guides/prompt-caching>
- OpenAI pricing:
  - <https://openai.com/api/pricing/>
- OpenAI GPT-5.3-Codex model page:
  - <https://developers.openai.com/api/docs/models/gpt-5.3-codex>
- Anthropic pricing:
  - <https://platform.claude.com/docs/en/about-claude/pricing>
- Anthropic prompt caching:
  - <https://platform.claude.com/docs/en/build-with-claude/prompt-caching>
- Gemini caching:
  - <https://ai.google.dev/gemini-api/docs/caching>
- Gemini pricing:
  - <https://ai.google.dev/gemini-api/docs/pricing>
- DeepSeek pricing:
  - <https://api-docs.deepseek.com/quick_start/pricing>
- DeepSeek context caching:
  - <https://api-docs.deepseek.com/guides/kv_cache>
- Moonshot / Kimi pricing:
  - <https://platform.kimi.ai/docs/pricing/chat-k2>
