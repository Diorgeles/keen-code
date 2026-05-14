## Pre-Approved Permissions

1. We have a task here: https://github.com/mochow13/keen-code/issues/17. Let's discuss.
2. What do you sugget for persistence?
3. Let's store the configs in project level. Allow/block will override existing permission mechanisms (but filesystem guards in place should still apply). So agent cannot access sensitive directories etc. I think during permission check, we just need to lookup the in-project config for these tools. If a tool is neither blocked nor allowed, normal Keen code mechanism will apply.  So we need a way to go back to original mechanism. I am thinking, maybe we introduce `/permission [allow|deny|default] <tool_names separated by space`. What do you think?
4. The config file will look like: { allow: { }, deny: { }} There shouldn't be any default key. If not present in one of these two, it's default. `/permission allow` still needs to respect filesystem guards for safety. Yeah let's load at startup and sync in-memory. But updates should also persist in the config file.
5. Let's create a plan and save in @.ai-interactions/tasks/issue-17/ as output-1_permissions.md.
6. I think I will simplify this:
  - `/allow-tool <tool_names>` to allow a tool always
  - `/reset-permission <tool_names>` to reset permissions for tools, meaning fallback to Keen's default permission mechanism. It means, we no longer have a `deny` option. It's either full allow (including dangerous commands for bash) or default which is Keen's behaviour.