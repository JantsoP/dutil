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
	Aliases     []string         // Aliases
	Description string           // Description for this container
	Children    []CommandHandler // Children

	DefaultHandler  CommandHandler // Ran when no sub command specified, prints help by default
	NotFoundHandler CommandHandler // Ran when sub command not found, by default it will print this containers help
}

func (cc *CommandContainer) GenerateHelp(target string, depth int) string {
	out := ""

	aliasesStr := ""
	if len(cc.Aliases) > 0 {
		aliasesStr = " (" + strings.Join(cc.Aliases, ",") + ")"

	}
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
				out += fmt.Sprintf("**%s**%s: %s\n", cc.Name, aliasesStr, cc.Description)
				if len(cc.Children) > 0 {
					for _, child := range cc.Children {
						out += child.GenerateHelp("", 0) + "\n"
					}
				}
			}
		}
	} else {
		out += fmt.Sprintf("**%s** %s: %s", cc.Name, aliasesStr, cc.Description)
		if depth > 0 && cc.Children != nil {
			for _, child := range cc.Children {
				out += "\n" + child.GenerateHelp("", depth-1)
			}
		}
	}

	return out
}

func (cc *CommandContainer) CheckMatch(raw string, source CommandSource, m *discordgo.MessageCreate, s *discordgo.Session) bool {
	fields := strings.SplitN(raw, " ", 2)
	if strings.EqualFold(fields[0], cc.Name) {
		return true
	}

	for _, alias := range cc.Aliases {
		if strings.EqualFold(fields[0], alias) {
			return true
		}
	}

	return false
}

func (cc *CommandContainer) HandleCommand(raw string, source CommandSource, m *discordgo.MessageCreate, s *discordgo.Session) error {
	split := strings.SplitN(raw, " ", 2)

	found := false
	if len(split) > 1 {
		for _, v := range cc.Children {
			if v.CheckMatch(split[1], source, m, s) {
				v.HandleCommand(split[1], source, m, s)
				found = true
				break
			}
		}

		if !found {
			if cc.NotFoundHandler != nil {
				cc.NotFoundHandler.HandleCommand(split[1], source, m, s)
			} else {
				cc.SendUnknownHelp(m, s)
			}
		}
	} else {
		if cc.DefaultHandler != nil {
			cc.DefaultHandler.HandleCommand("", source, m, s)
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
