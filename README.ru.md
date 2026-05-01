[Английская версия](README.ru.md)

Это форк **[comptplus](https://github.com/ionoscloudsdk/comptplus)**. Оригинальный пакет основывается на [go-prompt](https://github.com/elk-language/go-prompt), который как и оригинальный [go-prompt](https://github.com/c-bata/go-prompt) не поддерживает кастомную обработку прерываний. Этот форк решает эту проблему используя внутри **[go-prompt-ctrl-c](https://github.com/1ight181/go-prompt-ctrl-c)**, добавляющий новую опцию  `WithInterruptCallback` для конструктора, позволяющую передавать функцию, которая вызывается при сигналах прерывания. 
## Поддерживаемые сигналы

- `SIGINT`
- `SIGTERM`
- `SIGQUIT`
## Пример использования

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

// Кастомная обработка Ctrl+C
func interruptCallback(code int) {
	fmt.Println("\nCtrl+C pressed, but we handle it ourselves")
	// os.Exit(code) // Поведение по умолчанию 
}
```
## Сообщество 

Email: [danil.odinzov181@gmail.com](mailto:danil.odinzov181@gmail.com)