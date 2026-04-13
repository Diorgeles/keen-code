## Context Management

In Keen Code, we don't have any context management in place. We need to implement a fundamental context management system.

### Context Status UI

1. As the very first step, we will implement a simple context status UI for Keen Code. Below are the requirements:
  - The status  will be shown at the bottom of the screen under the input text area on the right side
  - It will have two parts: one progress bar and one percentage indicator
  - The status of the context will be determined based on the number of tokens in the current context size vs the context window of the current model. For example, if the current context size is 1000 tokens and the context window is 2000 tokens, the status will be 50%
  - The progress bar will be a horizontal bar with the current percentage filled
  - The percentage indicator will be a text showing the current percentage
  - The progress bar and the percentage indicator will be styled to match the theme of the UI
  - The progress bar and the percentage indicator will be updated in real-time as the conversation progresses based on the model's context window and the current context size
  - The progress bar and the percentage indicator will be updated in real-time as the model changes
  - We need to maintain a mapping between the model and its context size. This info can be maintained in @providers/registry.yaml for each model as a new field called `context_window`
  - To figure out the context size of a model, use web search
  - To determine the current context size, we need can use a simple assumption: 1 token is approximately 4 characters. So if there are 1000 words in the current conversation, then the current context size is 1000/0.75 = 1333 tokens.
  - Based on the requirements, create a plan for the implementation with granular todo items. Save the plan in @.ai-interactions/outputs/phase-4/output-6_context-status-ui.md.
2. Why are we rebuilding the conversation in @context_status.go using the currentContextStatus function? Why don't we just use the conversation history?
3. Actually, recalculating the context on every token is too much. Let's only update the status after each LLM message is done. So when LLM response finishes, we can update the status. It will reduce the number of times we rebuild the current context size.
4. Instead of coloring the bar for various percentages, we can color the percentage. So the bar will have one color.
5. Let's support context percentage up to 2 decimal points.

### Compaction

1. Right now, we don't have a way of compacting the running context. Let's brainstorm on this.
2. Figure out the compaction strategy in ../opencode. There is a compaction.ts file.
3. Ok based on our discussion, we have the following requirements for compaction:
  - To compact, we will send a request to the LLM to compact the context using a special system prompt for compaction. This call to LLM won't have any tools enabled.
  - The compaction process will preserve the last 20 messages (agent + user messages) in the conversation history. It's because latest messages are more important and likely to be relevant to the current task
  - Compaction process will be run on the conversation history stored in AppState.messages in @state.go
  - The compaction will create a summary of the conversation history including the last 20 messages and replace the current context with the summary + last 20 messages
  - The summary part will be considered as a single user message.
  - The summary has to be useful, contain important information about the conversation history, things like goals, discoveries, achieved milestones, and at the same time be concise
  - With a `/compact` command, the user can trigger a compaction of the running context
  - Users can also pass a prompt as an argument to the `/compact` command to specify how the compaction should be done. For example, user can say `/compact Keep the details about business logic`. Then the compaction will create a compacted context containing the summary of the conversation history, last 10 messages, and the details about business logic
  - If 70% of the context is filled, a suggestion will be shown to the user to compact the context alongside the existing compaction status UI
  - Compaction will NOT be triggered automatically
  - When compaction is either triggered manually, Keen Code will show a "<spinner> Compacting..." status to the user for the whole duration of the compaction process. It will be shown in place of loading spinner and loading text
  - Users can cancel the compaction process at any time by pressing the ESC key
  - When compaction is running, users cannot send any new messages. They have to wait for the compaction to finish
  - When compaction is finished, the conversation history will be replaced with the compacted context
  - After compaction is finished, compaction status UI will be updated accordingly
Based on the requirements, create a plan for implementing the compaction strategy. Make sure to include granular todo items for you in the plan. Save the plan in @.ai-interactions/outputs/phase-4/ as output-7_compaction.md.
4. I think we shouldn't have separate file for compaction tests. Let's move them to state.go.
5. What did you do in spinnerHeight function in internal/cli/repl/repl.go?
6. Then do we even need that function?
7. I mean, m.showSpinner is enough to show the spinner, now? Why have this separate function anymore?
8. In internal/cli/repl/state.go in the Compact function, are we skipping compaction if there are less than 20 messages?
9. I am thinking to not preserve last 20 messages at all. Let's just compact the whole thing and avoid tailing last 20 messages. Let's update it.
10. 