package dutil

import (
	"github.com/jonas747/discordgo"
	"os"
	"testing"
)

var (
	dgo *discordgo.Session // Stores global discordgo session

	envToken   = os.Getenv("DG_TOKEN")   // Token to use when authenticating
	envChannel = os.Getenv("DG_CHANNEL") // Channel ID to use for tests
)

func init() {
	if envToken == "" {
		return
	}

	if d, err := discordgo.New(envToken); err == nil {
		dgo = d
	}
}

func RequireSession(t *testing.T) bool {
	if dgo == nil || dgo.Token == "" {
		t.Skip("Not logged into discord, skipping...")
		return false
	}

	return true
}

func RequireTestingChannel(t *testing.T) bool {
	if !RequireSession(t) {
		return false
	}

	if envChannel == "" {
		t.Skip("No testing channel specified, skipping...")
		return false
	}

	return true
}
