# Adversary Feature — User Prompts

## Design & Planning

1. We have an idea to provide a `/second-opinion` feature in Keen as mentioned in @.ai-interactions/ideas/feature-exploration.md. What do you think of this?

2. What about `/second-opinion` only where the other model will respond to the last user message? It won't have the edit/write access. Perhaps no bash tool access either. If the main model changed something, `/second-opinion` can provide feedback. If it answered something, `/second-opinion` can put a different perspective and criticise the main answer.

3. We can allow `/second-opinion model` as a command to setup model. Users can also provide `/second-opinion <prompt>` to provide a specific guidance for providing opinions. The context of second opinion will be the full conversation so far. By default, it will provide opinion on the overall state of the ongoing task. Users can also ask specific part to provide feedback on using the prompt. What do you think?

4. We will not add any diff or file content. It will have access to read tools only. It can read as needed.

5. Interview me more on this.

    - Yes it will have a distinct label, exactly how `btw` is rendered. But we will use the secondaryColor.
    - Reuse existing interactive picker.
    - It starts fresh from the full main conversation. Doesn't take into account previous adversary turns.
    - Yes. Also, we want to separately store active model config for adversary.

9. Create a plan in @.ai-interactions/tasks/issue-52 as output-1_adversary.md

10. Why do we have to change @internal/cli/repl/widgets/model_selection?

11. Ok update the plan.

12. In `StreamAdversary`, we have an instruction. That should be updated. Perhaps a general 'act based on your responsibility' is enough?

13. If users don't provide a prompt, we don't need to show anything by default. Only the chip is enough.

14. Instead of adding all adversary fields flattened in replModel, perhaps we can declare a separate struct and add it to replModel?

15. Compact and build based on the plan in @.ai-interactions/tasks/issue-52/output-1_adversary.md

## Post-Implementation Refinements

16. Adversary shouldn't be invoked when agent is running already.

17. Adversary response should be brief and avoid bloat. Also adversary needs to be aware of the fact that its role is to check and critique main agent's work. It should refer it as such.

18. The adversary is thinking that it's checking/verifying its own work.

19. If user tries to send adversary command while main agent is streaming, we show a message now and empty the input text area. Let's just do nothing. Input text area doesn't empty, and no message is shown.
