## Tool Memory

### Ideation
At this stage, tool calls are only retained within the agent loop in a turn. After the turn, the tool call and its output are not retained. We want to avoid retaining tool calls and their outputs in the conversation history. But at the same time, past tool calls and their outputs can help the model to perform better in the subsequent turns.

1. How can we improve this without retaining tool input/outputs?
2. Can we only have key points after each agent turn? that can be an assistant segment? This is like a summary but only for the tool calls and their outputs.
3. What if we ask the LLM to give a tool memory but we don't show it in the REPL UI? Just store it in the conversation history.
4. Should we not have one memory after each turn?
5. What about storing only the latest 10 tool memories? We discard the older ones and don't send them to the LLM.

### PRD

Based on our discussion, this is the PRD for the tool memory feature:

1. After each agent turn is finished, the LLM will write a tool memory block at the very end of the turn from the tool usage in that turn
2. It will be instructed through the system prompt to write the tool memory block with the following fixed delimiters: <keen_memory>...</keen_memory>
3. Tool memory is a summary of the most important signals from the tool calls and their outputs
4. If no tool calls were made in a turn, LLM will write no tool memory
5. Tool memory won't be shown in the REPL UI. But it will be stored in the conversation history so that it can be in the context window of the LLM in the subsequent turns
6. We have to distinguish tool memory from other assistant messsages so that we can hide it in the REPL UI
7. Tool memory should summarize outcomes, not raw tool I/O.
8. Tool memory should be short, for example a few bullets or a small paragraph.
9. Session resume and compaction should preserve the retained tool memories.

Let's create an implementation plan for this feature based on the PRD. Save it in a file called `output-3_tool-memory.md` in @.ai-interactions/outputs/phase-5 directory.

### Follow Ups

1. We can simplify the design even further. Just emit the <keen_memory> blocks, and filter them out in REPL.  That's it. Since it's part of the conversation, it naturally gets into the context.
2. Update the plan to reflect the simplified design.
3. We have a bug. If <keen_memory> tag appears somewhere in the agent's message and not intended as tool memory, REPL still strips it.
4. `Right now, we are using <keen_memory> to emit tool memory from LLM in its turn for the whole turn so that the memory can be retained for later turns. The problem with this approach is that it is behaving very bad whenever any part of the LLM messages have <keen_memory> tag.
5. Where does it happen:  Persistence treats it as memory only when it is a dedicated trailing block
6. We should go for the long-term reliable solution. The one you proposed, explain more.
7. How can we deterministically create summary for tool usages, given that we need to be careful to retain the most important detail concisely?
8. This is too complex.
9. There can be many many tool calls, no?
10. Should we rather only retain memory for write_file, edit_file, and bash tool? Other tools are read-only, model can use them again if needed without side-effects.
11. Figuring out what bash facts to store will be complex. There can be so many different bash commands. I am thinking to leverage the \isDangerous` argument. If LLM marks a command as `isDangerous`, we can store it in the TurnMemory.`
12. Okay so based on our discussion, this is what we want to retain:
    - After every turn, we will create a tool memory object for the turn
    - The object will have all the written or edited files, deduplicated in the turn
    - It will also have bash command that failed with exit_code != 0
    - Bash commands will also have the associated exit_code retained, but not the output
    - We will store the TurnMemory in the session history in AppState.messages
    - While converting internal Message objects to OpenAI or Genkit messages, we will put it as a part of the assistant message content
What do you think of these requirements?
13. Ok now let's create an implementation plan and save it in .ai-interactions/outputs/phase-5 as output-4_tool-memory-redesign.md