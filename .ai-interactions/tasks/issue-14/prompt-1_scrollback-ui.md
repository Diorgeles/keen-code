# Scrollback and Text Selection UI

1. What can we do for this? https://github.com/mochow13/keen-code/issues/14 Suggest some solutions first that solves the problem but causes the least amount of changes.
2. So how do we scroll in option 1?
3. But terminal can support mouse wheel scrolling?
4. If a terminal supports mouse wheel scrolling and we choose option 1, then it's upto the terminal to support/not support mouse-wheel scrolling?
5. Let's first Disable mouse capture by default
6. Ok mouse wheel scrolling is translating to up/down in Ghostty terminal
7. But this solution is behaving differently in different terminals. And it's also buggy in iterm2.
8. Discard the current changes.
9. I think this is our best course of action: Stop relying on alt-screen
10. I tested it. The content can only be scrolled with page up and down. Though copy paste works.
11. If we keep streaming messages in bubble tea view and committed messages in scrollback, would it cause issues with the UI, like flickering or so?
12. How big can it be when streamin in bubble tea?
13. I am thinking we should perhaps implement our own copy-paste support inside the bubbletea viewport. Does bubbletea not support such a thing?
14. Ok we shouldn't implement our own copy
15. We need a long-term solution.
16. Ok let's work on this. Checkout to a new branch. Then proceed from what you did (git diff)
17. branch name: feat/scrollback-repl
18. When I write a message, since live stream is in viewport, a big gap appears first and content gets streamed into it. When finally stream finishes, there are ghost lines and spinner and input text area from the stream hanging.
19. The gap is still appearing. Now input text area is flickering between bottom and  right after the content of the agent messages.
20. Ok it's not working without bugs.
21. But if we remove bubble tea, we lose good UI
22. I found this from Crush, an AI coding agent: https://github.com/charmbracelet/crush/pull/563/changes This MR seems to support text selection. Check.
23. Ok now I have cloned the crush codebase in ../crush. Check it for the current text selection implementation. This was the previous PR: https://github.com/charmbracelet/crush/pull/563/changes
24. Ok let's work on this. I have discarded the changes we made.
25. selecting is automatically copying to clipboard. How can we show a message on the UI that it's copied?
26. It's getting out of the window if the window width is smaller. Can we show it just above the input text area? We can show it in place of loading text.
27. Perhaps we don't do it. When users select a section, they only select it. Users then need to use cmd+C or ctrl+c to copy. Let's do that.