# Cobra-Prompt Plus (Comptplus)

![Comptplus Banner](https://github.com/user-attachments/assets/2f7c9298-8f6e-4da5-940c-6f90ce285955)

> A production-ready fork of [Cobra Prompt](https://github.com/stromland/cobra-prompt) that turns any [Cobra](https://github.com/spf13/cobra) CLI into a rich interactive shell with syntax highlighting, fuzzy completions, and proper shell semantics.

Built on [elk-language/go-prompt](https://github.com/elk-language/go-prompt), an actively maintained fork of go-prompt with critical bug fixes (Ctrl+C handling, terminal restoration, grapheme support).

## Used by
- [IONOS Cloud CLI (ionosctl)](https://github.com/ionos-cloud/ionosctl/)

## What's different from cobra-prompt?

The original cobra-prompt is unmaintained and has several critical bugs. Comptplus fixes all of them and adds features needed for production CLIs:

- **Syntax highlighting**: commands, flags, values, and errors are colorized as you type via a cobra-aware lexer
- **Fuzzy completions**: type `dpl` to match `deploy`, powered by go-prompt's fuzzy filter
- **Flag value completions**: auto-complete flag values with caching and optional async (non-blocking) fetching
- **Shell-aware parsing**: quoted arguments (`--name "John Oliver"`) work correctly via go-shellquote
- **Proper cobra integration**: uses `SetArgs()` instead of `os.Args`, passes resolved `*cobra.Command` to callbacks
- **Graceful error handling**: errors print and return to the prompt instead of calling `os.Exit(1)`
- **Hooks and customization**: pre/post execution hooks, custom flag reset logic, dynamic prefix, key bindings, and more

See the [CHANGELOG](CHANGELOG.md) for the full version history.

## Getting started

```
go get github.com/ionoscloudsdk/comptplus
```

```go
package main

import (
    "github.com/elk-language/go-prompt"
    "github.com/ionoscloudsdk/comptplus"
)

func main() {
    lexer := comptplus.NewCobraLexer(rootCmd)

    cp := &comptplus.CobraPrompt{
        RootCmd:               rootCmd,
        AddDefaultExitCommand: true,
        FuzzyFilter:           true,
        GoPromptOptions: []prompt.Option{
            prompt.WithPrefix("> "),
            prompt.WithLexer(lexer),
        },
    }
    cp.Run()
}
```

## Try the example

```
go run ./_example
```
