# Prompts for /btw feature

1. Implement a `/btw <question>` feature — ephemeral side question to the LLM that doesn't pollute main conversation history. Read-only tools only (read_file, grep, glob). Separate system prompt. Stream the response.
2. Reuse existing `StreamChat` with a new `OneShot` flag to skip pending state — no new method on LLM clients.
3. Save implementation plan to `.ai-interactions/tasks/issue-39/output-1_btw.md`.
4. Use a box for rendering btw. Use `AccentColor` for borders and label. More padding on right and bottom.
5. Implement.
6. Allow `/btw` to be sent while main stream is active (separate stream handler, cancel, event channel).
7. Fix: btw stream prematurely ending after thinking tokens — skip ReasoningChunk events.
8. Btw viewport should accumulate all past Q&A (in-memory, session-local). `/btw` alone opens history if any exists, otherwise shows usage message.
9. Review the changes.
