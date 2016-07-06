package commandsystem

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dutil"
	"strings"
)

// Command container, you can nest these infinitely if you want
type CommandContainer struct {
	Name        string           // Name
	Description string           // Description for this container
	Children    []CommandHandler // Children

	DefaultHandler  CommandHandler // Ran when no sub command specified, prints help by default
	NotFoundHandler CommandHandler // Ran when sub command not found, by default it will print this containers help
}

func (cc *CommandContainer) GenerateHelp(target string, depth int) string {
	out := ""
	if target != "" {
		fields := strings.SplitN(target, " ", 2)
		if strings.EqualFold(fields[0], cc.Name) {
			// To further down the rabbit hole untill we find the proper commandhandler
			if len(fields) > 1 {
				if len(cc.Children) > 0 {
					for _, child := range cc.Children {
						out += child.GenerateHelp(fields[1], depth-1) + "\n"
					}
				} else {
					out += "Unknown command :'("
				}
			} else {
				// Show help for this container
				out += fmt.Sprintf("**%s**: %s\n", cc.Name, cc.Description)
				if len(cc.Children) > 0 {
					for _, child := range cc.Children {
						out += child.GenerateHelp("", 0) + "\n"
					}
				}
			}
		}
	} else {
		out += fmt.Sprintf(" - **%s**: %s", cc.Name, cc.Description)
		if depth > 0 && cc.Children != nil {
			for _, child := range cc.Children {
				out += "\n  - " + child.GenerateHelp("", depth-1)
			}
		}
	}

	return out
}

func (cc *CommandContainer) CheckMatch(raw string, m *discordgo.MessageCreate, s *discordgo.Session) bool {
	fields := strings.SplitN(raw, " ", 2)
	if strings.EqualFold(fields[0], cc.Name) {
		return true
	}

	return false
}

func (cc *CommandContainer) HandleCommand(raw string, m *discordgo.MessageCreate, s *discordgo.Session) error {
	split := strings.SplitN(raw, " ", 2)

	found := false
	if len(split) > 1 {
		for _, v := range cc.Children {
			if v.CheckMatch(split[1], m, s) {
				v.HandleCommand(split[1], m, s)
				found = true
				break
			}
		}

		if !found {
			if cc.NotFoundHandler != nil {
				cc.NotFoundHandler.HandleCommand(split[1], m, s)
			} else {
				cc.SendUnknownHelp(m, s)
			}
		}
	} else {
		if cc.DefaultHandler != nil {
			cc.DefaultHandler.HandleCommand("", m, s)
		} else {
			cc.SendUnknownHelp(m, s)
		}
	}
	return nil
}

func (cc *CommandContainer) SendUnknownHelp(m *discordgo.MessageCreate, s *discordgo.Session) {
	helpStr := cc.GenerateHelp("", 1)
	dutil.SplitSendMessage(s, m.ChannelID, "**Unknown command, Help:**\n"+helpStr)
}
