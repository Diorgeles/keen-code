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