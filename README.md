[Русская версия](README.ru.md)

This is a fork of **[comptplus](https://github.com/ionoscloudsdk/comptplus)**. The original package is based on [go-prompt](https://github.com/elk-language/go-prompt), which—like the original [go-prompt](https://github.com/c-bata/go-prompt)—does not support custom interrupt handling. This fork solves that issue by using **[go-prompt-ctrl-c](https://github.com/1ight181/go-prompt-ctrl-c)** internally, which introduces a new constructor option `WithInterruptCallback`. This option allows you to pass a function that will be invoked when interrupt signals are received.
## Supported Signals

- `SIGINT`
- `SIGTERM`
- `SIGQUIT`
## Usage Example

```go
package main

import (
	"fmt"

	cobraprompt "github.com/1ight181/comptplus-ctrl-c"
	prompt "github.com/1ight181/go-prompt-ctrl-c"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		SilenceUsage:  true,
		SilenceErrors: true,
		Short:         "root cmd example",
	}

	cobraPrompt := cobraprompt.CobraPrompt{
		RootCmd:                 rootCmd,
		ShowHelpCommandAndFlags: true,
		GoPromptOptions: []prompt.Option{
			prompt.WithTitle("example 0.1"),
			prompt.WithMaxSuggestion(5),
			prompt.WithPrefix("example> "),
			prompt.WithInterruptCallback(interruptCallback),
		},
	}

	cobraPrompt.Run()
}

// Custom Ctrl+C handling
func interruptCallback(code int) {
	fmt.Println("\nCtrl+C pressed, but we handle it ourselves")
	// os.Exit(code) // Default behavior
}
```
## Community

Email: [danil.odinzov181@gmail.com](mailto:danil.odinzov181@gmail.com)