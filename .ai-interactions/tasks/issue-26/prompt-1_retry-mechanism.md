## Retry LLM Invocation

### OpenAI and OpenAI Codex

1. We have an implementation of retry based on exponential backoff in @internal/llm/openai.go. We need to implement the same in @internal/llm/openai_codex.go and @internal/llm/openai_responses.go. Let's create a todo list before writing any code.
2. We want to inject maxRetries so that we can pass a different value. Right now, it's set to 10 on package level which is why tests run for longer.
3. Simulate retry for @internal/llm/openai_codex.go. How can we do it by locally spinning up a server and passing it as openAiCodexBaseURL?
4. Similarly, let's update both @internal/llm/anthropic.go and @internal/llm/genkit.go with retries.