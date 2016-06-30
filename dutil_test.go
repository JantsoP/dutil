package dutil

import (
	"github.com/bwmarrin/discordgo"
	"os"
)

var (
	dg *discordgo.Session // Stores global discordgo session

	envToken   = os.Getenv("DG_TOKEN")   // Token to use when authenticating
	envChannel = os.Getenv("DG_CHANNEL") // Channel ID to use for tests
)

func init() {
	if envToken == "" {
		return
	}

	if d, err := discordgo.New(envToken); err == nil {
		dg = d
	}
}
