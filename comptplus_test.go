package comptplus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/elk-language/go-prompt"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindSuggestions(t *testing.T) {
	rootCmd := newTestCommand("root", "The root cmd")
	getCmd := newTestCommand("get", "Get something")
	getObjectCmd := newTestCommand("object", "Get the object")
	getThingCmd := newTestCommand("thing", "The thing")
	getFoodCmd := newTestCommand("food", "Get some food")
	getFoodCmd.PersistentFlags().StringP("name", "n", "John", "name of the person to get some food from")
	_ = getFoodCmd.RegisterFlagCompletionFunc("name", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"John", "Mary\tMarianne - John's Mother", "Anne"}, cobra.ShellCompDirectiveNoFileComp
	})

	rootCmd.AddCommand(getCmd)
	getCmd.AddCommand(getObjectCmd, getThingCmd, getFoodCmd)
	getObjectCmd.Flags().BoolP("verbose", "v", false, "Verbose log")

	cp := &CobraPrompt{
		RootCmd: rootCmd,
	}

	tests := []struct {
		name            string
		input           string
		expectedResults []string
	}{
		{
			name:            "Root suggestions",
			input:           "",
			expectedResults: []string{"get"},
		},
		{
			name:            "Get command suggestions",
			input:           "get ",
			expectedResults: []string{"object", "food", "thing"},
		},
		{
			name:            "Verbose flag suggestions",
			input:           "get object -",
			expectedResults: []string{"-v"},
		},
		{
			name:            "Verbose long flag suggestions",
			input:           "get object --",
			expectedResults: []string{"--verbose"},
		},
		{
			name:            "Name flag suggestions after flag",
			input:           "get food --name ",
			expectedResults: []string{"John", "Mary", "Anne"},
		},
		{
			name:            "Name flag suggestions with partial value",
			input:           "get food --name J",
			expectedResults: []string{"John"},
		},
		{
			name:            "Shorthand name flag suggestions",
			input:           "get food -n ",
			expectedResults: []string{"John", "Mary", "Anne"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := prompt.NewBuffer()
			buf.InsertTextMoveCursor(test.input, 999, 999, false)
			suggestions, _, _ := cp.findSuggestions(*buf.Document())

			assert.Len(t, suggestions, len(test.expectedResults), "Incorrect number of suggestions")

			actualSuggestionsMap := make(map[string]struct{})
			for _, suggestion := range suggestions {
				actualSuggestionsMap[suggestion.Text] = struct{}{}
			}

			// Check each expected result is present in actual suggestions
			for _, expected := range test.expectedResults {
				_, exists := actualSuggestionsMap[expected]
				assert.True(t, exists, "Expected suggestion not found: "+expected)
			}
		})
	}
}

func TestAsyncFlagValueSuggestions(t *testing.T) {
	rootCmd := newTestCommand("root", "The root cmd")
	getCmd := newTestCommand("get", "Get something")
	getFoodCmd := newTestCommand("food", "Get some food")
	getFoodCmd.PersistentFlags().StringP("name", "n", "John", "name of the person")

	fetches := make(chan struct{}, 10)
	_ = getFoodCmd.RegisterFlagCompletionFunc("name", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		fetches <- struct{}{}
		return []string{"John", "Mary\tMarianne", "Anne"}, cobra.ShellCompDirectiveNoFileComp
	})

	rootCmd.AddCommand(getCmd)
	getCmd.AddCommand(getFoodCmd)

	cp := &CobraPrompt{
		RootCmd:                   rootCmd,
		AsyncFlagValueSuggestions: true,
	}

	// First call: returns empty, triggers background fetch
	buf := prompt.NewBuffer()
	buf.InsertTextMoveCursor("get food --name ", 999, 999, false)
	suggestions, _, _ := cp.findSuggestions(*buf.Document())
	assert.Empty(t, suggestions, "First async call should return empty")

	// Wait for background goroutine to complete
	select {
	case <-fetches:
	case <-time.After(time.Second):
		t.Fatal("background fetch did not complete")
	}
	// Small sleep to let the goroutine finish writing to cache after the fetch
	time.Sleep(10 * time.Millisecond)

	// Second call: returns cached results
	suggestions, _, _ = cp.findSuggestions(*buf.Document())
	assert.Len(t, suggestions, 3)
	texts := make(map[string]struct{})
	for _, s := range suggestions {
		texts[s.Text] = struct{}{}
	}
	assert.Contains(t, texts, "John")
	assert.Contains(t, texts, "Mary")
	assert.Contains(t, texts, "Anne")

	// Verify only one fetch occurred (cache should still be fresh)
	suggestions, _, _ = cp.findSuggestions(*buf.Document())
	assert.Len(t, suggestions, 3)
	assert.Len(t, fetches, 0, "Should not have triggered another fetch while cache is fresh")
}

// TestParseInput validates shellquote-based parsing, fallback, and custom parsers.
func TestParseInput(t *testing.T) {
	t.Run("quoted arguments are preserved as single tokens", func(t *testing.T) {
		cp := &CobraPrompt{RootCmd: newTestCommand("root", "")}
		args := cp.parseInput(`get --name "John Oliver" food`)
		assert.Equal(t, []string{"get", "--name", "John Oliver", "food"}, args)
	})

	t.Run("single-quoted arguments", func(t *testing.T) {
		cp := &CobraPrompt{RootCmd: newTestCommand("root", "")}
		args := cp.parseInput(`get --name 'John Oliver' food`)
		assert.Equal(t, []string{"get", "--name", "John Oliver", "food"}, args)
	})

	t.Run("unclosed quote falls back to strings.Fields", func(t *testing.T) {
		cp := &CobraPrompt{RootCmd: newTestCommand("root", "")}
		args := cp.parseInput(`get --name "John Oliver`)
		// strings.Fields fallback: splits naively including the dangling quote
		assert.Equal(t, strings.Fields(`get --name "John Oliver`), args)
	})

	t.Run("empty input returns nil", func(t *testing.T) {
		cp := &CobraPrompt{RootCmd: newTestCommand("root", "")}
		args := cp.parseInput("")
		assert.Empty(t, args)
	})

	t.Run("custom InArgsParser overrides default", func(t *testing.T) {
		cp := &CobraPrompt{
			RootCmd: newTestCommand("root", ""),
			InArgsParser: func(input string) []string {
				return []string{"custom", "parsed"}
			},
		}
		args := cp.parseInput("anything here")
		assert.Equal(t, []string{"custom", "parsed"}, args)
	})

	t.Run("backslash-escaped spaces", func(t *testing.T) {
		cp := &CobraPrompt{RootCmd: newTestCommand("root", "")}
		args := cp.parseInput(`get --name John\ Oliver food`)
		assert.Equal(t, []string{"get", "--name", "John Oliver", "food"}, args)
	})
}

// TestExecuteCommand_SetArgs verifies commands use SetArgs instead of os.Args.
// This is the fix from PR #6 — comptplus must work as a sub-command of a larger CLI.
func TestExecuteCommand_SetArgs(t *testing.T) {
	var receivedArgs []string
	rootCmd := &cobra.Command{
		Use:           "root",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	echoCmd := &cobra.Command{
		Use: "echo",
		RunE: func(cmd *cobra.Command, args []string) error {
			receivedArgs = args
			return nil
		},
	}
	rootCmd.AddCommand(echoCmd)

	cp := newCobraPromptForTest(rootCmd)

	executor := cp.executeCommand(context.Background())
	executor("echo hello world")

	assert.Equal(t, []string{"hello", "world"}, receivedArgs)
}

// TestExecuteCommand_QuotedArgs ensures shell-quoted arguments reach the command correctly.
func TestExecuteCommand_QuotedArgs(t *testing.T) {
	var receivedArgs []string
	rootCmd := &cobra.Command{
		Use:           "root",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	echoCmd := &cobra.Command{
		Use: "echo",
		RunE: func(cmd *cobra.Command, args []string) error {
			receivedArgs = args
			return nil
		},
	}
	rootCmd.AddCommand(echoCmd)

	cp := newCobraPromptForTest(rootCmd)

	executor := cp.executeCommand(context.Background())
	executor(`echo "hello world" foo`)

	assert.Equal(t, []string{"hello world", "foo"}, receivedArgs)
}

// TestHookBefore_AbortsPreventsExecution ensures a failing HookBefore stops command execution.
func TestHookBefore_AbortsExecution(t *testing.T) {
	executed := false
	rootCmd := &cobra.Command{
		Use:           "root",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	doCmd := &cobra.Command{
		Use: "do",
		Run: func(cmd *cobra.Command, args []string) {
			executed = true
		},
	}
	rootCmd.AddCommand(doCmd)

	hookErr := errors.New("auth required")
	var capturedErr error

	cp := newCobraPromptForTest(rootCmd)
	cp.HookBefore = func(_ *cobra.Command, _ string) error { return hookErr }
	cp.OnErrorFunc = func(err error) { capturedErr = err }

	executor := cp.executeCommand(context.Background())
	executor("do")

	assert.False(t, executed, "command should not have executed when HookBefore returns error")
	assert.Equal(t, hookErr, capturedErr)
}

// TestHookAfter_ErrorIsReported ensures HookAfter errors are routed to OnErrorFunc.
func TestHookAfter_ErrorIsReported(t *testing.T) {
	rootCmd := &cobra.Command{
		Use:           "root",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	doCmd := &cobra.Command{
		Use: "do",
		Run: func(cmd *cobra.Command, args []string) {},
	}
	rootCmd.AddCommand(doCmd)

	afterErr := errors.New("post-exec cleanup failed")
	var capturedErr error

	cp := newCobraPromptForTest(rootCmd)
	cp.HookAfter = func(_ *cobra.Command, _ string) error { return afterErr }
	cp.OnErrorFunc = func(err error) { capturedErr = err }

	executor := cp.executeCommand(context.Background())
	executor("do")

	assert.Equal(t, afterErr, capturedErr)
}

// TestHookBefore_ReceivesResolvedCommand verifies the hook gets the resolved sub-command, not root.
func TestHookBefore_ReceivesResolvedCommand(t *testing.T) {
	rootCmd := &cobra.Command{
		Use:           "root",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	subCmd := &cobra.Command{
		Use: "sub",
		Run: func(cmd *cobra.Command, args []string) {},
	}
	rootCmd.AddCommand(subCmd)

	var hookCmd *cobra.Command
	cp := newCobraPromptForTest(rootCmd)
	cp.HookBefore = func(cmd *cobra.Command, _ string) error { hookCmd = cmd; return nil }

	executor := cp.executeCommand(context.Background())
	executor("sub")

	require.NotNil(t, hookCmd)
	assert.Equal(t, "sub", hookCmd.Use)
}

// TestResetFlagsToDefault validates flag reset after command execution.
func TestResetFlagsToDefault(t *testing.T) {
	t.Run("string flag resets to default", func(t *testing.T) {
		rootCmd := &cobra.Command{
			Use:           "root",
			SilenceErrors: true,
			SilenceUsage:  true,
		}
		doCmd := &cobra.Command{
			Use: "do",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		doCmd.Flags().StringP("output", "o", "table", "output format")
		rootCmd.AddCommand(doCmd)

		cp := newCobraPromptForTest(rootCmd)

		executor := cp.executeCommand(context.Background())
		executor("do --output json")

		val, _ := doCmd.Flags().GetString("output")
		assert.Equal(t, "table", val, "flag should reset to default after execution")
	})

	t.Run("slice flag resets without appending to previous values", func(t *testing.T) {
		rootCmd := &cobra.Command{
			Use:           "root",
			SilenceErrors: true,
			SilenceUsage:  true,
		}
		doCmd := &cobra.Command{
			Use: "do",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		doCmd.Flags().StringSlice("tags", []string{"default"}, "tags to apply")
		rootCmd.AddCommand(doCmd)

		cp := newCobraPromptForTest(rootCmd)

		executor := cp.executeCommand(context.Background())

		// Execute twice with different values
		executor("do --tags=alpha --tags=beta")
		tags, _ := doCmd.Flags().GetStringSlice("tags")
		assert.Equal(t, []string{"default"}, tags, "tags should reset to [default] after first exec")

		executor("do --tags=gamma")
		tags, _ = doCmd.Flags().GetStringSlice("tags")
		assert.Equal(t, []string{"default"}, tags, "tags should reset to [default] after second exec, not accumulate")
	})
}

// TestPersistFlagValues verifies flags are NOT reset when PersistFlagValues is true.
func TestPersistFlagValues(t *testing.T) {
	rootCmd := &cobra.Command{
		Use:           "root",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	doCmd := &cobra.Command{
		Use: "do",
		Run: func(cmd *cobra.Command, args []string) {},
	}
	doCmd.Flags().StringP("output", "o", "table", "output format")
	rootCmd.AddCommand(doCmd)

	cp := newCobraPromptForTest(rootCmd)
	cp.PersistFlagValues = true

	executor := cp.executeCommand(context.Background())
	executor("do --output json")

	val, _ := doCmd.Flags().GetString("output")
	assert.Equal(t, "json", val, "flag should persist when PersistFlagValues is true")
}

// TestOnErrorFunc_DefaultPrintsToStderr ensures errors go to stderr when no OnErrorFunc is set.
func TestOnErrorFunc_DefaultPrintsToStderr(t *testing.T) {
	var stderr bytes.Buffer
	rootCmd := &cobra.Command{
		Use:           "root",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	rootCmd.SetErr(&stderr)

	failCmd := &cobra.Command{
		Use: "fail",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("something broke")
		},
	}
	rootCmd.AddCommand(failCmd)

	cp := newCobraPromptForTest(rootCmd)
	// No OnErrorFunc — should print to stderr, not os.Exit

	executor := cp.executeCommand(context.Background())
	executor("fail")

	assert.Contains(t, stderr.String(), "something broke")
}

// TestDynamicSuggestions_ReceivesResolvedCommand validates the PR #6 signature change:
// DynamicSuggestionsFunc now receives the resolved *cobra.Command, enabling ValidArgsFunction usage.
func TestDynamicSuggestions_ReceivesResolvedCommand(t *testing.T) {
	rootCmd := newTestCommand("root", "")
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List items",
		Run:   func(cmd *cobra.Command, args []string) {},
		Annotations: map[string]string{
			DynamicSuggestionsAnnotation: "items",
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{"alpha\tFirst item", "beta\tSecond item"}, cobra.ShellCompDirectiveNoFileComp
		},
	}
	rootCmd.AddCommand(listCmd)

	var receivedCmd *cobra.Command
	var receivedAnnotation string

	cp := &CobraPrompt{
		RootCmd: rootCmd,
		DynamicSuggestionsFunc: func(cmd *cobra.Command, annotation string, doc *prompt.Document) []prompt.Suggest {
			receivedCmd = cmd
			receivedAnnotation = annotation

			// Use ValidArgsFunction like PR #6 demonstrated
			var suggestions []prompt.Suggest
			if cmd.ValidArgsFunction != nil {
				completions, _ := cmd.ValidArgsFunction(cmd, []string{}, "")
				for _, c := range completions {
					text, desc, _ := strings.Cut(c, "\t")
					suggestions = append(suggestions, prompt.Suggest{Text: text, Description: desc})
				}
			}
			return suggestions
		},
	}

	buf := prompt.NewBuffer()
	buf.InsertTextMoveCursor("list ", 999, 999, false)
	suggestions, _, _ := cp.findSuggestions(*buf.Document())

	require.NotNil(t, receivedCmd)
	assert.Equal(t, "list", receivedCmd.Use, "DynamicSuggestionsFunc should receive the resolved command")
	assert.Equal(t, "items", receivedAnnotation)
	assert.Len(t, suggestions, 2)
	assert.Equal(t, "alpha", suggestions[0].Text)
	assert.Equal(t, "First item", suggestions[0].Description)
}

// TestHiddenCommandsAndFlags verifies visibility toggles.
func TestHiddenCommandsAndFlags(t *testing.T) {
	rootCmd := newTestCommand("root", "")
	visibleCmd := newTestCommand("visible", "A visible command")
	hiddenCmd := newTestCommand("secret", "A hidden command")
	hiddenCmd.Hidden = true

	flagCmd := newTestCommand("flagged", "Has hidden flags")
	flagCmd.Flags().String("public", "", "public flag")
	flagCmd.Flags().String("internal", "", "internal flag")
	flagCmd.Flags().MarkHidden("internal")

	rootCmd.AddCommand(visibleCmd, hiddenCmd, flagCmd)

	t.Run("hidden commands excluded by default", func(t *testing.T) {
		cp := &CobraPrompt{RootCmd: rootCmd}
		suggestions := suggestTexts(cp, "")
		assert.Contains(t, suggestions, "visible")
		assert.NotContains(t, suggestions, "secret")
	})

	t.Run("hidden commands included when ShowHiddenCommands is true", func(t *testing.T) {
		cp := &CobraPrompt{RootCmd: rootCmd, ShowHiddenCommands: true}
		suggestions := suggestTexts(cp, "")
		assert.Contains(t, suggestions, "visible")
		assert.Contains(t, suggestions, "secret")
	})

	t.Run("hidden flags excluded by default", func(t *testing.T) {
		cp := &CobraPrompt{RootCmd: rootCmd}
		suggestions := suggestTexts(cp, "flagged --")
		assert.Contains(t, suggestions, "--public")
		assert.NotContains(t, suggestions, "--internal")
	})

	t.Run("hidden flags included when ShowHiddenFlags is true", func(t *testing.T) {
		cp := &CobraPrompt{RootCmd: rootCmd, ShowHiddenFlags: true}
		suggestions := suggestTexts(cp, "flagged --")
		assert.Contains(t, suggestions, "--public")
		assert.Contains(t, suggestions, "--internal")
	})
}

// TestBoolFlagDoesNotSuggestValues ensures bool flags don't trigger value completions.
func TestBoolFlagDoesNotSuggestValues(t *testing.T) {
	rootCmd := newTestCommand("root", "")
	cmd := newTestCommand("do", "Do something")
	cmd.Flags().BoolP("verbose", "v", false, "verbose output")
	cmd.Flags().StringP("format", "f", "text", "output format")
	_ = cmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "yaml", "text"}, cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.AddCommand(cmd)

	cp := &CobraPrompt{RootCmd: rootCmd}

	// After a bool flag, should show commands/flags, not values
	suggestions := suggestTexts(cp, "do --verbose ")
	assert.NotContains(t, suggestions, "true")
	assert.NotContains(t, suggestions, "false")

	// After a string flag, should show values
	suggestions = suggestTexts(cp, "do --format ")
	assert.Contains(t, suggestions, "json")
	assert.Contains(t, suggestions, "yaml")
}

// TestFlagValueDescriptions verifies tab-separated descriptions in completions.
func TestFlagValueDescriptions(t *testing.T) {
	rootCmd := newTestCommand("root", "")
	cmd := newTestCommand("deploy", "Deploy")
	cmd.Flags().String("region", "", "target region")
	_ = cmd.RegisterFlagCompletionFunc("region", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"us-east-1\tVirginia",
			"eu-west-1\tIreland",
			"ap-south-1\tMumbai",
		}, cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.AddCommand(cmd)

	cp := &CobraPrompt{RootCmd: rootCmd}

	buf := prompt.NewBuffer()
	buf.InsertTextMoveCursor("deploy --region ", 999, 999, false)
	suggestions, _, _ := cp.findSuggestions(*buf.Document())

	require.Len(t, suggestions, 3)

	descMap := make(map[string]string)
	for _, s := range suggestions {
		descMap[s.Text] = s.Description
	}
	assert.Equal(t, "Virginia", descMap["us-east-1"])
	assert.Equal(t, "Ireland", descMap["eu-west-1"])
	assert.Equal(t, "Mumbai", descMap["ap-south-1"])
}

// TestInheritedFlags ensures inherited (persistent) flags from parents are suggested on child commands.
func TestInheritedFlags(t *testing.T) {
	rootCmd := newTestCommand("root", "")
	rootCmd.PersistentFlags().StringP("output", "o", "table", "output format")

	childCmd := newTestCommand("child", "A child command")
	rootCmd.AddCommand(childCmd)

	cp := &CobraPrompt{RootCmd: rootCmd}

	suggestions := suggestTexts(cp, "child --")
	assert.Contains(t, suggestions, "--output", "inherited persistent flag should be suggested on child")
}

// TestSuggestionFilter verifies custom suggestion filters override the default.
func TestSuggestionFilter(t *testing.T) {
	rootCmd := newTestCommand("root", "")
	rootCmd.AddCommand(newTestCommand("alpha", "Alpha cmd"))
	rootCmd.AddCommand(newTestCommand("beta", "Beta cmd"))
	rootCmd.AddCommand(newTestCommand("gamma", "Gamma cmd"))

	cp := &CobraPrompt{
		RootCmd: rootCmd,
		SuggestionFilter: func(suggestions []prompt.Suggest, doc *prompt.Document) []prompt.Suggest {
			// Only return suggestions starting with 'b'
			var filtered []prompt.Suggest
			for _, s := range suggestions {
				if strings.HasPrefix(s.Text, "b") {
					filtered = append(filtered, s)
				}
			}
			return filtered
		},
	}

	suggestions := suggestTexts(cp, "")
	assert.Equal(t, []string{"beta"}, suggestions)
}

// TestAsyncFlagValueSuggestions_ConcurrentAccess stress-tests the flag cache under concurrent reads.
func TestAsyncFlagValueSuggestions_ConcurrentAccess(t *testing.T) {
	rootCmd := newTestCommand("root", "")
	cmd := newTestCommand("do", "Do it")
	cmd.Flags().String("env", "", "environment")
	_ = cmd.RegisterFlagCompletionFunc("env", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		time.Sleep(5 * time.Millisecond) // simulate slow fetch
		return []string{"dev", "staging", "prod"}, cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.AddCommand(cmd)

	cp := &CobraPrompt{
		RootCmd:                   rootCmd,
		AsyncFlagValueSuggestions: true,
	}

	// Fire multiple concurrent findSuggestions calls
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := prompt.NewBuffer()
			buf.InsertTextMoveCursor("do --env ", 999, 999, false)
			cp.findSuggestions(*buf.Document())
		}()
	}
	wg.Wait()

	// After all goroutines settle, the cache should have the results
	time.Sleep(50 * time.Millisecond)

	buf := prompt.NewBuffer()
	buf.InsertTextMoveCursor("do --env ", 999, 999, false)
	suggestions, _, _ := cp.findSuggestions(*buf.Document())
	assert.Len(t, suggestions, 3)
}

// TestFlagValueCache_Invalidation verifies cache expires after the interval.
func TestFlagValueCache_Invalidation(t *testing.T) {
	rootCmd := newTestCommand("root", "")
	cmd := newTestCommand("do", "Do it")
	cmd.Flags().Duration(CacheIntervalFlag, 50*time.Millisecond, "cache interval")
	cmd.Flags().String("env", "", "environment")

	callCount := 0
	_ = cmd.RegisterFlagCompletionFunc("env", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		callCount++
		return []string{fmt.Sprintf("result-%d", callCount)}, cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.AddCommand(cmd)

	cp := &CobraPrompt{RootCmd: rootCmd}

	buf := prompt.NewBuffer()
	buf.InsertTextMoveCursor("do --env ", 999, 999, false)

	// First fetch
	suggestions, _, _ := cp.findSuggestions(*buf.Document())
	require.Len(t, suggestions, 1)
	assert.Equal(t, "result-1", suggestions[0].Text)

	// Immediate second fetch should use cache
	suggestions, _, _ = cp.findSuggestions(*buf.Document())
	assert.Equal(t, "result-1", suggestions[0].Text, "should still be cached")

	// Wait for cache to expire
	time.Sleep(60 * time.Millisecond)

	suggestions, _, _ = cp.findSuggestions(*buf.Document())
	require.Len(t, suggestions, 1)
	assert.Equal(t, "result-2", suggestions[0].Text, "cache should have expired, triggering a new fetch")
}

// TestPrepareCommands verifies exit command and help command setup.
func TestPrepareCommands(t *testing.T) {
	t.Run("exit command added when AddDefaultExitCommand is true", func(t *testing.T) {
		rootCmd := newTestCommand("root", "")
		cp := &CobraPrompt{
			RootCmd:               rootCmd,
			AddDefaultExitCommand: true,
		}
		cp.prepareCommands()

		found := false
		for _, c := range rootCmd.Commands() {
			if c.Use == "exit" {
				found = true
				break
			}
		}
		assert.True(t, found, "exit command should be added")
	})

	t.Run("completion command disabled when DisableCompletionCommand is true", func(t *testing.T) {
		rootCmd := newTestCommand("root", "")
		cp := &CobraPrompt{
			RootCmd:                  rootCmd,
			DisableCompletionCommand: true,
		}
		cp.prepareCommands()
		assert.True(t, rootCmd.CompletionOptions.DisableDefaultCmd)
	})
}

// TestCustomFlagResetBehaviour verifies custom reset logic is used instead of default.
func TestCustomFlagResetBehaviour(t *testing.T) {
	rootCmd := &cobra.Command{
		Use:           "root",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd := &cobra.Command{
		Use: "do",
		Run: func(cmd *cobra.Command, args []string) {},
	}
	cmd.Flags().String("mode", "auto", "operating mode")
	rootCmd.AddCommand(cmd)

	resetCalled := false
	cp := newCobraPromptForTest(rootCmd)
	cp.CustomFlagResetBehaviour = func(flag *pflag.Flag) {
		resetCalled = true
		// Custom: always reset to "manual" instead of the actual default
		flag.Value.Set("manual")
	}

	executor := cp.executeCommand(context.Background())
	executor("do --mode turbo")

	assert.True(t, resetCalled)
	val, _ := cmd.Flags().GetString("mode")
	assert.Equal(t, "manual", val)
}

// TestDeepCommandTree validates suggestions work through deeply nested command hierarchies.
func TestDeepCommandTree(t *testing.T) {
	rootCmd := newTestCommand("root", "")
	l1 := newTestCommand("level1", "First level")
	l2 := newTestCommand("level2", "Second level")
	l3 := newTestCommand("level3", "Third level")
	l3.Flags().String("deep-flag", "", "a deep flag")

	rootCmd.AddCommand(l1)
	l1.AddCommand(l2)
	l2.AddCommand(l3)

	cp := &CobraPrompt{RootCmd: rootCmd}

	assert.Contains(t, suggestTexts(cp, ""), "level1")
	assert.Contains(t, suggestTexts(cp, "level1 "), "level2")
	assert.Contains(t, suggestTexts(cp, "level1 level2 "), "level3")
	assert.Contains(t, suggestTexts(cp, "level1 level2 level3 --"), "--deep-flag")
}

// TestCommandAliases verifies that aliases resolve for suggestion navigation.
func TestCommandAliases(t *testing.T) {
	rootCmd := newTestCommand("root", "")
	getCmd := &cobra.Command{
		Use:     "get",
		Aliases: []string{"g", "fetch"},
		Short:   "Get resources",
		Run:     func(cmd *cobra.Command, args []string) {},
	}
	itemCmd := newTestCommand("item", "An item")
	getCmd.AddCommand(itemCmd)
	rootCmd.AddCommand(getCmd)

	cp := &CobraPrompt{RootCmd: rootCmd}

	// Alias should resolve and show subcommands
	assert.Contains(t, suggestTexts(cp, "g "), "item")
	assert.Contains(t, suggestTexts(cp, "fetch "), "item")
}

// TestExecuteCommand_ContextPropagation verifies the context is passed through to commands.
func TestExecuteCommand_ContextPropagation(t *testing.T) {
	type ctxKey string
	rootCmd := &cobra.Command{
		Use:           "root",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	var receivedVal string
	cmd := &cobra.Command{
		Use: "check",
		Run: func(cmd *cobra.Command, args []string) {
			if v, ok := cmd.Context().Value(ctxKey("test")).(string); ok {
				receivedVal = v
			}
		},
	}
	rootCmd.AddCommand(cmd)

	cp := newCobraPromptForTest(rootCmd)

	ctx := context.WithValue(context.Background(), ctxKey("test"), "hello-from-context")
	executor := cp.executeCommand(ctx)
	executor("check")

	assert.Equal(t, "hello-from-context", receivedVal)
}

// --- Helpers ---

// newCobraPromptForTest creates a CobraPrompt with all hooks/reset behaviour initialized,
// mirroring what RunContext does. Use this for tests that call executeCommand directly.
func newCobraPromptForTest(rootCmd *cobra.Command) *CobraPrompt {
	return &CobraPrompt{
		RootCmd:    rootCmd,
		HookBefore: func(_ *cobra.Command, _ string) error { return nil },
		HookAfter:  func(_ *cobra.Command, _ string) error { return nil },
		CustomFlagResetBehaviour: func(flag *pflag.Flag) {
			sliceValue, ok := flag.Value.(pflag.SliceValue)
			if !ok {
				flag.Value.Set(flag.DefValue)
				return
			}
			defValue := strings.Trim(flag.DefValue, "[]")
			defaultSlice := strings.Split(defValue, ",")
			if err := sliceValue.Replace(defaultSlice); err != nil {
				if errEmpty := sliceValue.Replace([]string{}); errEmpty == nil {
					flag.Value.Set(flag.DefValue)
				}
			}
		},
	}
}

func newTestCommand(use string, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Run:   func(cmd *cobra.Command, args []string) {},
	}
}

// suggestTexts is a helper that returns just the .Text values from findSuggestions.
func suggestTexts(cp *CobraPrompt, input string) []string {
	buf := prompt.NewBuffer()
	buf.InsertTextMoveCursor(input, 999, 999, false)
	suggestions, _, _ := cp.findSuggestions(*buf.Document())
	var texts []string
	for _, s := range suggestions {
		texts = append(texts, s.Text)
	}
	return texts
}
