## Roadmap Going Forward

1. In keen code, what new features should we support to make it realistically useful for software engineers and be competitive with other CLI-based coding agents like OpenCode, Claude Code, or Codex?
2. We should also support agent skills. Keen Code will expose the capabilities for users to add their own skills to the agent.
3. For the item number 2, we will have a "autopilot" mode only. Existing permission mechanism will remain as it is. If users go for autopilot mode, all permissions will be bypassed (except for bash tool). What do you think of this?
4. For item 3, is it really needed? Engineers can always rely on Git for controlling versions and undoing/rewinding changes.
5. I have also changed my mind. I think we should ship some default skills along with skills infra. That way, we can easily implement various skills out of the box, like /revert, /review, etc. What do you think?
6. Well I change my mind again. Let's remove skills altogether for now. We will think about it later.