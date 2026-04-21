package comptplus

import (
	"testing"

	"github.com/elk-language/go-prompt"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLexerTestTree() *cobra.Command {
	root := &cobra.Command{Use: "root", SilenceErrors: true, SilenceUsage: true}

	server := &cobra.Command{Use: "server", Short: "Manage servers", Run: func(cmd *cobra.Command, args []string) {}}
	serverList := &cobra.Command{Use: "list", Short: "List servers", Run: func(cmd *cobra.Command, args []string) {}}
	serverList.Flags().StringP("output", "o", "table", "output format")
	serverList.Flags().BoolP("verbose", "v", false, "verbose")
	server.PersistentFlags().String("datacenter-id", "", "datacenter UUID")
	server.AddCommand(serverList)

	deploy := &cobra.Command{Use: "deploy", Short: "Deploy app", Aliases: []string{"dpl"}, Run: func(cmd *cobra.Command, args []string) {}}
	deploy.Flags().String("env", "", "environment")

	root.AddCommand(server, deploy)
	return root
}

// collectTokens runs the lexer and returns all tokens as (text, color) pairs.
func collectTokens(l *CobraLexer, input string) []struct {
	text  string
	color prompt.Color
} {
	l.Init(input)
	var result []struct {
		text  string
		color prompt.Color
	}
	for {
		tok, ok := l.Next()
		if !ok {
			break
		}
		first := int(tok.FirstByteIndex())
		last := int(tok.LastByteIndex()) + 1 // LastByteIndex is inclusive
		result = append(result, struct {
			text  string
			color prompt.Color
		}{
			text:  input[first:last],
			color: tok.Color(),
		})
	}
	return result
}

func TestCobraLexer_Commands(t *testing.T) {
	root := newLexerTestTree()
	l := NewCobraLexer(root)

	t.Run("single command is green", func(t *testing.T) {
		tokens := collectTokens(l, "server")
		require.Len(t, tokens, 1)
		assert.Equal(t, "server", tokens[0].text)
		assert.Equal(t, prompt.Green, tokens[0].color)
	})

	t.Run("nested command path is green", func(t *testing.T) {
		tokens := collectTokens(l, "server list")
		colors := tokenColors(tokens)
		assert.Equal(t, prompt.Green, colors["server"])
		assert.Equal(t, prompt.Green, colors["list"])
	})

	t.Run("alias is green", func(t *testing.T) {
		tokens := collectTokens(l, "dpl")
		require.Len(t, tokens, 1)
		assert.Equal(t, prompt.Green, tokens[0].color)
	})
}

func TestCobraLexer_Flags(t *testing.T) {
	root := newLexerTestTree()
	l := NewCobraLexer(root)

	t.Run("valid long flag is cyan", func(t *testing.T) {
		tokens := collectTokens(l, "server list --output")
		colors := tokenColors(tokens)
		assert.Equal(t, prompt.Cyan, colors["--output"])
	})

	t.Run("valid short flag is cyan", func(t *testing.T) {
		tokens := collectTokens(l, "server list -o")
		colors := tokenColors(tokens)
		assert.Equal(t, prompt.Cyan, colors["-o"])
	})

	t.Run("inherited flag is cyan", func(t *testing.T) {
		tokens := collectTokens(l, "server list --datacenter-id")
		colors := tokenColors(tokens)
		assert.Equal(t, prompt.Cyan, colors["--datacenter-id"])
	})

	t.Run("unknown flag is red", func(t *testing.T) {
		tokens := collectTokens(l, "server list --nonexistent")
		colors := tokenColors(tokens)
		assert.Equal(t, prompt.Red, colors["--nonexistent"])
	})
}

func TestCobraLexer_FlagValues(t *testing.T) {
	root := newLexerTestTree()
	l := NewCobraLexer(root)

	t.Run("value after string flag is yellow", func(t *testing.T) {
		tokens := collectTokens(l, "server list --output json")
		colors := tokenColors(tokens)
		assert.Equal(t, prompt.Cyan, colors["--output"])
		assert.Equal(t, prompt.Yellow, colors["json"])
	})

	t.Run("value after short flag is yellow", func(t *testing.T) {
		tokens := collectTokens(l, "server list -o json")
		colors := tokenColors(tokens)
		assert.Equal(t, prompt.Yellow, colors["json"])
	})

	t.Run("value after bool flag is not yellow", func(t *testing.T) {
		tokens := collectTokens(l, "server list --verbose something")
		colors := tokenColors(tokens)
		// "something" is after a bool flag, so it's a positional arg (default color), not a value
		assert.Equal(t, prompt.DefaultColor, colors["something"])
	})
}

func TestCobraLexer_UnknownTokens(t *testing.T) {
	root := newLexerTestTree()
	l := NewCobraLexer(root)

	t.Run("unknown command is default color", func(t *testing.T) {
		tokens := collectTokens(l, "bogus")
		require.Len(t, tokens, 1)
		assert.Equal(t, prompt.DefaultColor, tokens[0].color)
	})

	t.Run("positional arg after command is default", func(t *testing.T) {
		tokens := collectTokens(l, "deploy myapp")
		colors := tokenColors(tokens)
		assert.Equal(t, prompt.Green, colors["deploy"])
		assert.Equal(t, prompt.DefaultColor, colors["myapp"])
	})
}

func TestCobraLexer_QuotedValues(t *testing.T) {
	root := newLexerTestTree()
	l := NewCobraLexer(root)

	t.Run("double-quoted flag value is yellow", func(t *testing.T) {
		tokens := collectTokens(l, `deploy --env "prod east"`)
		colors := tokenColors(tokens)
		assert.Equal(t, prompt.Cyan, colors["--env"])
		assert.Equal(t, prompt.Yellow, colors[`"prod east"`])
	})

	t.Run("single-quoted flag value is yellow", func(t *testing.T) {
		tokens := collectTokens(l, `deploy --env 'staging'`)
		colors := tokenColors(tokens)
		assert.Equal(t, prompt.Yellow, colors["'staging'"])
	})
}

func TestCobraLexer_EmptyInput(t *testing.T) {
	root := newLexerTestTree()
	l := NewCobraLexer(root)

	l.Init("")
	_, ok := l.Next()
	assert.False(t, ok)
}

func TestCobraLexer_WhitespacePreserved(t *testing.T) {
	root := newLexerTestTree()
	l := NewCobraLexer(root)

	tokens := collectTokens(l, "server  list")
	// Should be: "server", "  " (whitespace), "list"
	require.Len(t, tokens, 3)
	assert.Equal(t, "server", tokens[0].text)
	assert.Equal(t, "  ", tokens[1].text)
	assert.Equal(t, "list", tokens[2].text)
}

func TestCobraLexer_CustomColors(t *testing.T) {
	root := newLexerTestTree()
	l := NewCobraLexer(root)
	l.CommandColor = prompt.Blue
	l.FlagColor = prompt.Fuchsia
	l.ValueColor = prompt.Turquoise
	l.ErrorColor = prompt.Brown

	tokens := collectTokens(l, "server list --output json --bogus")
	colors := tokenColors(tokens)
	assert.Equal(t, prompt.Blue, colors["server"])
	assert.Equal(t, prompt.Blue, colors["list"])
	assert.Equal(t, prompt.Fuchsia, colors["--output"])
	assert.Equal(t, prompt.Turquoise, colors["json"])
	assert.Equal(t, prompt.Brown, colors["--bogus"])
}

// tokenColors builds a map of word -> color, skipping whitespace tokens.
func tokenColors(tokens []struct {
	text  string
	color prompt.Color
}) map[string]prompt.Color {
	m := make(map[string]prompt.Color)
	for _, t := range tokens {
		if t.text != "" && t.text != " " && t.text != "  " {
			m[t.text] = t.color
		}
	}
	return m
}
