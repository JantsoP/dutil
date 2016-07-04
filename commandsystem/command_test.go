package commandsystem

import (
	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseCommand(t *testing.T) {

	testCommand := &CommandDef{
		Name: "Poo",
		Arguments: []*ArgumentDef{
			&ArgumentDef{Name: "Str", Type: ArgumentTypeString},
			&ArgumentDef{Name: "Num", Type: ArgumentTypeNumber},
		},
	}
	testStr := "Poo teststr 123"

	parsed, err := ParseCommand(testStr, testCommand, &discordgo.MessageCreate{&discordgo.Message{Content: testStr}}, nil)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "teststr", parsed.Args[0].Str(), "Should match")
	assert.Equal(t, 123, parsed.Args[1].Int(), "Should match")
}
