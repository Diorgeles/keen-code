## Supporing Z.ai GLM Models

1. We have an issue on Github: https://github.com/mochow13/keen-code/issues/3 for supporting Z.ai GLM models. Check the issue for more details. Then create an implementation plan for supporting Z.ai GLM models. Save the plan in the `.ai-interactions/tasks/issue-3/output-1_glm-model-plan.md` file.
2. For GLM we also need to support thinking. Check this example of a cURL request to the Z.ai GLM API:

```bash
curl --location 'https://api.z.ai/api/paas/v4/chat/completions' \
--header 'Authorization: Bearer YOUR_API_KEY' \
--header 'Content-Type: application/json' \
--data '{
    "model": "glm-5",
    "messages": [
        {
            "role": "user",
            "content": "Explain in detail the basic principles of quantum computing and analyze its potential impact in the field of cryptography"
        }
    ],
    "thinking": {
        "type": "enabled"
    },
    "max_tokens": 4096,
    "temperature": 1.0
}'
```