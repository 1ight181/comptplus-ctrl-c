package main

import (
	"os"
	"strings"

	"github.com/1ight181/go-prompt-ctrl-c"
	cobraprompt "github.com/ionoscloudsdk/comptplus"
	"github.com/ionoscloudsdk/comptplus/_example/cmd"
	"github.com/spf13/cobra"
)

var lexer = cobraprompt.NewCobraLexer(cmd.RootCmd)

var advancedPrompt = &cobraprompt.CobraPrompt{
	RootCmd:                  cmd.RootCmd,
	PersistFlagValues:        true,
	ShowHelpCommandAndFlags:  true,
	DisableCompletionCommand: true,
	AddDefaultExitCommand:    true,
	FuzzyFilter:              true,
	CompletionOnDown:         true,
	GoPromptOptions: []prompt.Option{
		prompt.WithTitle("cobra-prompt"),
		prompt.WithPrefix(">(^!^)> "),
		prompt.WithMaxSuggestion(10),
		prompt.WithLexer(lexer),
	},
	DynamicSuggestionsFunc: func(_ *cobra.Command, annotationValue string, document *prompt.Document) []prompt.Suggest {
		if suggestions := cmd.GetFoodDynamic(annotationValue); suggestions != nil {
			return suggestions
		}

		return []prompt.Suggest{}
	},
	OnErrorFunc: func(err error) {
		if strings.Contains(err.Error(), "unknown command") {
			cmd.RootCmd.PrintErrln(err)
			return
		}

		cmd.RootCmd.PrintErr(err)
		os.Exit(1)
	},
}

var simplePrompt = &cobraprompt.CobraPrompt{
	RootCmd:                  cmd.RootCmd,
	AddDefaultExitCommand:    true,
	DisableCompletionCommand: true,
}

func main() {
	// Change to simplePrompt to see the difference
	advancedPrompt.Run()
}
