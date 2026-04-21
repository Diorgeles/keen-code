 ## Context Status Redesign

 ### Plan

1. In context status, should we consider the agent loop's context that gets passed to the agent in the loop? Right now it doesn't.
2. What files did you change?
3. So explain how the context status is calculated right now
4. does providers provide token count and/or context usage?
5. does openai api or genkit or anthropic sdk support context usage metrics or something similar?
6. How can I use the provider's data to estimate context in use? Only explain without writing any code.
7. before proceeding, let's first revert the last changes you made to context status
8. Now create a plan for supporting context status using provider provided data, instead of existing local estimation. Save the plan in .ai-interactions/outputs/phase-5 as output-7_context-status-redesign.md

### Plan Review

1. In the plan .ai-interactions/outputs/phase-5/output-7_context-status-redesign.md, help me understand lastUsage vs nextCount
2. The next request will also include the user's message, right?
3. When the repl starts, what would the context used show?
4. What should we do if some providers don't provide the data?
5. What if we just show the lastUsage only? Do we strictly need "idle-state status"?
6. Ok update the plan. Also we can show "0.0%" instead of N/A

### Debug
1. We have implemented the changes required for .ai-interactions/outputs/phase-5/output-7_context-status-redesign.md. Review the changes. It seems context in use is not getting updated. Figure out why. Don't edit.
2. It's not particularly working for anthropic.
3. Explore anthropic api more and figure out if there is a different way to know input tokens etc.