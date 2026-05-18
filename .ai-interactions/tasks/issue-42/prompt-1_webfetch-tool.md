## Support `web_fetch` Tool

- We want to support web_fetch tool for Keen. The idea is simple: it receives a URL and returns the content.
- Can we consider converting pages to markdown? Is that feasible given that URLs may have javascript, html, images etc?
- Save the plan in @.ai-interactions/tasks/issue-42/ as output-1_webfetch-tool.md
- How many tokens in 512KB?
- Is it not too big given that many LLMs have less than 300k context limit?
- How many tokens in 128KB?
- Which version of `htmltomarkdown` are you using? Why not the latest one?
