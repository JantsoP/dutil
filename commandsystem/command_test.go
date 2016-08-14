package commandsystem

import (
	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseCommand(t *testing.T) {
	// Test normal command behaviour
	testCommand := &SimpleCommand{
		Name: "testcmd",
		Arguments: []*ArgumentDef{
			&ArgumentDef{Name: "Str", Type: ArgumentTypeString},
			&ArgumentDef{Name: "Num", Type: ArgumentTypeNumber},
		},
	}

	testStr := "testcmd teststr 123"
	parsed, err := testCommand.ParseCommand(testStr, &discordgo.MessageCreate{&discordgo.Message{Content: testStr}}, nil)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "teststr", parsed.Args[0].Str(), "Should match")
	assert.Equal(t, 123, parsed.Args[1].Int(), "Should match")
}

func TestParseCommandRequireArgs(t *testing.T) {
	// Test normal command behaviour
	testCommand := &SimpleCommand{
		Name:         "testcmd",
		RequiredArgs: 2,
		Arguments: []*ArgumentDef{
			&ArgumentDef{Name: "Str", Type: ArgumentTypeString},
			&ArgumentDef{Name: "Num", Type: ArgumentTypeNumber},
		},
	}

	testStr := "testcmd teststr"
	_, err := testCommand.ParseCommand(testStr, &discordgo.MessageCreate{&discordgo.Message{Content: testStr}}, nil)
	if !assert.Error(t, err) {
		return
	}
	// assert.Equal(t, "teststr", parsed.Args[0].Str(), "Should match")
	// assert.Equal(t, 123, parsed.Args[1].Int(), "Should match")
}

func TestParseCommandQouted(t *testing.T) {
	// Test normal command behaviour
	testCommand := &SimpleCommand{
		Name: "testcmd",
		Arguments: []*ArgumentDef{
			&ArgumentDef{Name: "Str", Type: ArgumentTypeString},
			&ArgumentDef{Name: "Str2", Type: ArgumentTypeString},
		},
	}

	testStr := "testcmd \"teststr 1\" \"teststr\\\" 2\""
	parsed, err := testCommand.ParseCommand(testStr, &discordgo.MessageCreate{&discordgo.Message{Content: testStr}}, nil)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "teststr 1", parsed.Args[0].Str(), "Should match")
	assert.Equal(t, "teststr\" 2", parsed.Args[1].Str(), "Should match")
}

func TestParseCommandSpace(t *testing.T) {
	// Test normal command behaviour
	testCommand := &SimpleCommand{
		Name: "testcmd",
		Arguments: []*ArgumentDef{
			&ArgumentDef{Name: "Str", Type: ArgumentTypeString},
			&ArgumentDef{Name: "Str2", Type: ArgumentTypeString},
		},
	}

	testStr := "testcmd teststr1\\ still\\ here second\\ arg"
	parsed, err := testCommand.ParseCommand(testStr, &discordgo.MessageCreate{&discordgo.Message{Content: testStr}}, nil)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "teststr1 still here", parsed.Args[0].Str(), "Should match")
	assert.Equal(t, "second arg", parsed.Args[1].Str(), "Should match")
}

func TestParseCommandCombo(t *testing.T) {
	testArgumentCombo := &SimpleCommand{
		Name: "testcmd",
		Arguments: []*ArgumentDef{
			&ArgumentDef{Name: "Str", Type: ArgumentTypeString},
			&ArgumentDef{Name: "Num", Type: ArgumentTypeNumber},
		},
		ArgumentCombos: [][]int{[]int{0, 1}, []int{1, 0}},
	}

	comboStr1 := "testcmd teststr 123"
	comboStr2 := "testcmd 123 teststr"

	testStr := "testcmd teststr 123"
	parsed, err := testArgumentCombo.ParseCommand(comboStr1, &discordgo.MessageCreate{&discordgo.Message{Content: testStr}}, nil)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "teststr", parsed.Args[0].Str(), "combo1 Should match")
	assert.Equal(t, 123, parsed.Args[1].Int(), "combo1  Should match")

	parsed, err = testArgumentCombo.ParseCommand(comboStr2, &discordgo.MessageCreate{&discordgo.Message{Content: testStr}}, nil)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "teststr", parsed.Args[0].Str(), "combo2 Should match")
	assert.Equal(t, 123, parsed.Args[1].Int(), "combo2 Should match")
}
