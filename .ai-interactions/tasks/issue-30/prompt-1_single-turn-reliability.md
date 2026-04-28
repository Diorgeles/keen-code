# Prompts: Single-Turn Reliability

1. So in `internal/llm/genkit.go`, `internal/llm/anthropic.go`, `internal/llm/openai_codex.go`, `internal/llm/openai.go` and `internal/llm/openai_responses.go` we have agent loops. But agent loop may fail suddenly. As a result, we lose all the work because we don't retain the raw tool transcript. As a solution, I am thinking of separately storing a CurrentToolState object. This will hold all the tool calls currently running. If the turn fails, the object will keep holding the tool calls and outputs. When the next turn starts, this CurrentToolState object will be passed to the agent. The moment a turn succeeds (specifically, agent sends a done message), we clear the CurrentToolState. The idea is to be failure-proof during a single turn. What do you think of this approach? Check the files first. Then think about it. Then explain. DO NOT WRITE ANY CODE.
2. I don't want to update Message or StreamChat. What if we store it as a state for each client integration? And separately inject tool data if any.
3. We will retain all tool calls, agent messages, and tool outputs when there is a failure or interruption by the user. In these cases, we shouldn't add the latest assistant messages to the Message object that holds the conversation history. Because these are already available in the pending state. What do you think?
4. Is that enough to put it just before the last user message? What if there are 3 consecutive user messages in between?
5. But how is that even possible to have two consecutive user messages?
6. Ok we will keep it simple.
7. What are the various states an agent's turn may end up to? Check all the files I mentioned where we have agent loops.
8. And what will happen to AppState's Message?
9. I prefer number 1. Would it work for all 5 cases?
10. Ok that's not a big deal
11. StreamEventTypeError can happen in the middle of a turn.
12. Awesome. Write a design doc (RFC) based on what we discussed. Save it in `.ai-interactions/tasks/issue-30` as `output-1_single-turn-reliability-rfc.md`
13. In the design, you mentioned checking for tool data accumulation. We not only accumulate tool data but also any assistante messages within the turn. Did we not discuss this in the prior discussion?
14. User interruption happens through Esc key.
15. Note that collectTurnWithRetry emits events and that can happen before StreamChat being able to check for accumulated data and returning proper event. We need to move event all emission from collectTurnWithRetry to StreamChat. What do you think?
16. Retry events can remain in collectTurnWithRetry because that doesn't impact the state.
