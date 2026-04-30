## REPL Refactoring

1. We need to work on the issue: https://github.com/mochow13/keen-code/issues/29. Check it first. Then create a plan for me. Save the plan in @.ai-interactions/tasks/issue-29/ as output-1_repl-refactor-plan.md
2. Checkout to a new branch.
3. Figure out if the refactor could cause side-effects, for example circular dependency issues.
4. Ok let's implement
5. Go through the code you edited again and review.
6. The dispatchCommand function in @internal/cli/repl/command_handlers.go: use switch instead of ifs. I expect no problem with that.
7. @internal/cli/repl/repl.go is a big file. List down all the functions that are helpers or auxliary functions that we can move to a different file, like utils.go or helpers.go.
8. Are there any side-effects if we move the "Simple model methods" to a separate file named repl_helpers.go?
9. Ok review all the refactors you have done so far. Basically review all the changes in this branch including the untracked files.