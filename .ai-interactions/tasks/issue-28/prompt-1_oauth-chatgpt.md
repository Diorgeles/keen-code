## Supporting OAuth for ChatGPT

1. Currently, we only support API key for OpenAI GPT models. We want to support OAuth for ChatGPT subscribed for Plus and Pro plans. The behaviour of the product will be like this:
    - When users configure a model using `/model` command, the will see a new option to select "OpenAI (ChatGPT Plus/Pro)" as the provider.
    - The other option will be "OpenAI (API Key)".
    - When users select "OpenAI (ChatGPT Plus/Pro)", the will be prompted to click on a link to authenticate with ChatGPT.
    - The link will be opened in the default browser.
    - Then users will authenticate with ChatGPT and finally when it's successful, they will see a message on the browser that the authentication is successful. They will be asked to go back to the CLI.
    - After that, users will be able to select the model they want to use. But users will not be prompted for baseURL or API key.
What do you think about this? Don't add any code yet. Let's discuss.
2. Explore the oauth integration in ../opencode. Check how they have implemented the oauth integration for OpenAI.
3. Write an RFC for implementing OAuth support for ChatGPT accounts. Save the design in .ai-interactions/tasks/issue-28 as output-1_chatgpt-oauth-rfc.md.  Include diagrams and explanations. At the end of the document, include fine- grained todo list for completing this task.
4. We won't support headless like opencode does. Remove it from the RFC.
5. We should keep the oauth flow and the API key flow separate.
6. We should keep a separate provider for the oauth flow. It is `chatgpt-codex`.
7. Figure out exact context window and thinking effort levels for the models that we plan to support for Codex.
8. Does the ChatGPT/Codex endpoint enforce lower plan-specific context limits for Plus than the static registry can represent?
9. Does the Codex OAuth client accept a dynamic localhost port, or must Keen Code use a fixed callback port?
10. Ok that's all. Let's implement the RFC: .ai-interactions/tasks/issue-28/output-1_chatgpt-oauth-rfc.md
11. Should handleLogout be in internal/cli/repl/handlers.go?
12. Ok let's create an issue on Github and label it as "refactor"
13. OAuth browser/callback error with `unexpected end of JSON input`.
14. I think we should keep internal/llm/openai_responses.go separate...
15. There are still changes in internal/llm/openai_responses.go and internal/llm/openai_responses_test.go
16. Whenever I am asking openai-codex to review code, it stops the stream without printing anything. Looks like Codex flow is not able to use models correctly.
17. It seems the issue is codex is not able to use tools.
18. We will rename: OpenAI (API Key) to OpenAI only. And OpenAI (ChatGPT/Codex) to Codex (ChatGPT OAuth).
19. Explain step by step by pointing out the code how ChatGPT OAuth is working. Start from the command /model. Explicitly lay out the functions.
20. The writeOAuthPage function has a too long line on line 333. Let's beautify it.