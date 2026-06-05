# In-Turn Context Reduction

1. During a turn, agent loop accumulate contexts. For example, in Keen, it happens in @internal/llm/genkit.go @internal/llm/openai.go @internal/llm/anthropic.go @internal/llm/openai_responses.go @internal/llm/openai_codex.go 
Right now if a tool turn is too long, context keeps accumulating and eventually grows even bigger than the allowed context window of the specific model. As a result, the following request fails.
We need to solve this problem. What are some possible approaches? Let's discuss first.
2. So can we run the whole context through the helpers before each request to models and simply truncate their on the fly? What do you think?
3. For now, we can only remove oldest tool results first. That could be our first step.
4. Ok for now, here's what we will do:
- Run each context through the context reducer
- If it goes beyond the context window of the model, we will replace olderst tool call results with placeholder: "Tool results removed to..." - don't make the placeholder long. 1/2 line is enough. We don't need to nudge to reuse the tool. Agents know it.
- If current estimated token count in the context is within the context window, we can stop reducing.
- We should dynamically estimate how much we have reduced so that we can directly deduce running length of context window in O(1).
What do you think?
5. From each provider, we know the last total input token size from the previous call.
Can we use it to estimate next request's context window size?
6. Is token count always length/4? is this estimate reliable?
7. Ok let's use provider's input token count where available
Can we implement this whole thing in separate files and simply call those functions where needed?
8. I think from each provider file, we can send the last input token counts so that reducer can use it. Each provider file can have its own small function that simply delegates the actual reduction to the context_reducer.
9. Is this really the simplest solution keeping our requirements intact?
10. No what I want is this:
- each provider passes the last request's context size along with the next request's full context that's about to be sent to LLM
- reducer only estimates the size of the delta between this and the last request
- reducer estimates the total context size based on baseline and the delta's estimation
- reducer then reduces oldest tool results with the placeholder and maintains a running length of the context
- if the length is below the threshold (model's context window size - our reserve buffer), then reducer stops
We don't want to change much in existing providers.
Reducer can have seprate functions for reducing per provider.
11. "Reduced context must become the new baseline" - it will happen automatically since it is sent to the providers and they return input token count
12. The reserve buffer we have, we should calculate that based on the current system prompts, tools we have. Can you figure it out already?
13. What's the safety margin for?
14. "For providers with explicit output limits, use the configured/provider-specific value where practical." actually we should use 8192 for each since it's unlikely anthropic will provide 64k tokens in one go for some task.
15. Ok let's implement
16. Context window size for model can be derived from @providers/registry.yaml. We should update the ClientConfig struct in @internal/llm/models.go to store the context window size in-memory. In this ClientConfig struct, we can have a field. Then in the NewClient function itself, we can read the @providers/registry.yaml to populate the correct context size.
17. Let's make the error message a constant and reuse it in @internal/llm/context_reducer.go 
18. We have over complicated this. We don't need to estimate token count for delta. We can simply reduce context if required after the end of LLM call. For example, in @openai.go, we can do it after preparing the `oaiMessages` slice for next call (after appending assistant message and tool messages)
This way, we can just pass the full oaiMessages to reducer along with returned input token size from the provider. The reducer simply checks against budget and replaces oldest tool results with the placeholder.
19. @internal/llm/context_reducer.go can be further refactored by moving common parts into separate functions.
20. For variable names, if a variable is a count, we should name it as count. For example, lastInputTokens > lastInputTokenCount
21. For target.tokenCount, we are estimating the count. I wonder if there is a better way (don't edit code yet)
22. For codex, we can call the same function `reduceResponsesContextForRequest` instead of a separate wrapper
23. Write useful unit tests for @internal/llm/context_reducer.go, don't bloat it. Avoid useless/obvious tests. 
24. Are the tests covering the functionalities of the reducer?
25. We are checking for budget in reduceResponsesContextForRequest, which is quite late.
This check should be moved right when providers call their respective reducers.
This check can be a separate function.
26. contextFitsBudget can be called in the reduce functions' beginning instead of inside providers
27. Hmm it's better to actually have the fit check in providers. It seems simpler.
28. Only on critical parts of the implementation for debugging purpose, add slog.Debug
29. Should we embed the registry so that it's always available when built?
30. Don't do file system lookup, use the embedded registry
31. Can we confirm we are loading correct context window size for a model? Are the unit tests catching this behaviour well?
32. We should make fallback context window to 200k
33. Check @internal/config/loader.go it already has GetModelContextWindow - why don't we use it instead of contextWindowForProviderModel?
34. Do we then need ContextWindowTokens in ClientConfig?
35. Do we need @providers/registry.go ?
