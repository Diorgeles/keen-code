## Input Navigation through Arrow Keys

1. Ok we need to work on issue #4 here: https://github.com/mochow13/keen-code/issues/4. Based on the requirements, create a plan to implement the input history navigation. Save the plan in @.ai-interactions/tasks/issue-4/output-1_input-history-nav-plan.md
2. In @.ai-interactions/tasks/issue-4/output-1_input-history-nav-plan.md, what will happen if a new session is created or an old one loaded back?
3. Implement the plan.
4. Can we move input_history and its test files to a subpackage named "history"?
5. Can we put the cursor at the end of each input when navigating history?
6. How are we ensuring the history file doesn't contain more than 1000 inputs from the user?
7. Let's rewrite history upon successful exit. There we can ensure we only retain the last 1000 inputs from the user.