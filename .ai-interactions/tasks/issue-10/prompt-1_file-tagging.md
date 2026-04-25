## File Tagging

1. Check the issue on Github: https://github.com/mochow13/keen-code/issues/10. First, let's discuss the problem and different solutions on how to approach this.
2. Below is the behaviour I would like to have for file tagging:
  - Users will use `@` to tag a file
  - Suggestions will only show first 10 matches
  - Suggestions will only be shown if user writes at least 1 character after `@`
  - Users can use up and down arrow keys to navigate through the suggestions
  - The selected suggestion will be highlighted in the REPL just like how it's done for command suggestions
  - Reuse existing code as much as possible to avoid code duplication
  - When user selects a suggestion, the relative path will be inserted into the REPL
  - Ask me any questions you have about the requirements
  - Users can tag multiple files by using `@` multiple times
Create a plan for the implementation. Save the plan in the `.ai-interactions/tasks/issue-10/output-1_file-tagging-plan.md` file.