## UI Enhancements

1. Right now when a diff is shown, it doesn't have a border to clearly separate it from the rest of the output. Let's add a border to the diff.
2. Instead of a box, let's show two lines: above and below. The box looks a bit weird.
3. In which places in the UI we show borders? Should we rather show such rules instead of boxes? It looks good for the diffs.
4. Let's add rules in bash.
5. Let's also have rules for the input text area instead of a box.
6. In tools where a path is provided as input, we show the full path in the UI. This is redundant. Let's show only the relative path to the working directory. This is just for the UI.
7. We currently show provider in secondary color. Let's change that. We will show the model in secondary color only.
8. Remove the progress bar and icon from the context status UI. Replace it with so

## Context Status
1. Currently, system prompt + tool schemas aren't fully accounted for in the percentage. The estimate includes llm.Build(workingDir) text but not the tool descriptors Genkit attaches under the hood. Real context pressure is higher than the bar suggests.

## Compaction
1. Currently, when a compaction happens, the compaction is not shown to client. We want to stream the compaction just like other assistant messages. How to do that?
2. `handleCompactionDone` and `handleCompactionError` - should they be in repl.go? I think we should put them in handlers.go.
3. Compacted context is not being replayed correctly in case of session resume. The resumed session only shows the "Context compacted." message but not the actual compactioned context. Figure out how to fix this.

## Thinking
1. Currently, there is no way to configure thinking effort in the CLI. We want to add two features:

- During setup of models using the `/model` users can select the thinking effort level
- At runtime, users can change the thinking effort level using the `/thinking` command

Note that not all models or providers support thinking. We should only show the thinking effort level selection if the model supports it. Figure out how to do it. You might want to search the web for more information about this. Note that supported models in Keen Code are in @registry.yaml file.

Use the AskUserQuestion tool to clarify further requirements.

2. What's the standard way of selecting a budget and map it to low/medium/high?
3. For anthropic, we can by default have adaptive but take the effort as a param. we can still name it as thinking
  param
4. Save the plan in @.ai-interactions/outputs/phase-5/output-5_thinking-param.md

## Migrating to `anthropic-sdk-go`

1. Currently, we are using genkit for anthropic models. Let's change that. We should use the official anthropic-sdk-go for those models. We already have the go module added to the project. But it's not the latest. We should migrate those models using the latest sdk.
2. It seems first chunk of thinking blocks are getting printed twice. Figure out why and fix it.

## KV Cache
1. We don't retain tool calls and their outputs beyond a single agent turn. For example, in internal/llm/openai.go, tool calls are apended to the openai messages within a single turn. But when the turn finishes, we don't retain the tool calls and outputs. Rather, we have only TurnMemory that's materliased at the end of each turn. Now the question is, how does it impact the KV cache, hence the cost?
2. What if we enable prompt caching for anthropic?
3. Looking at the code, where should we put the cache_control? Let's update the code accordingly.

## Refactoring

1. Repl package is full of files. How would you reorganise it? Save the plan in @.ai-interactions/outputs/phase-5/output-8_refactor.md
2. How can we make sure we don't break anything while doing this?