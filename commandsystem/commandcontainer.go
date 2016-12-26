package commandsystem

import (
	"context"
	"fmt"
	"github.com/bwmarrin/discordgo"
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

func (cc *CommandContainer) GenerateHelp(target string, maxDepth, currentDepth int) string {
	out := ""

	containerHelp := cc.ContainerHelp(currentDepth)

	if target != "" {
		fields := strings.SplitN(target, " ", 2)
		// Not the command being targetted
		if !strings.EqualFold(fields[0], cc.Name) {
			return ""
		}

		out += containerHelp

		// To further down the rabbit hole untill we find the proper commandhandler
		if len(fields) > 1 {
			for _, child := range cc.Children {
				childHelp := child.GenerateHelp(fields[1], maxDepth, currentDepth+1) + "\n"
				if strings.TrimSpace(childHelp) != "" {
					out += "\n" + childHelp
				}
			}
		} else {
			// Show help for this container
			if len(cc.Children) > 0 {
				for _, child := range cc.Children {
					childHelp := child.GenerateHelp("", maxDepth, currentDepth+1) + "\n"
					if strings.TrimSpace(childHelp) != "" {
						out += "\n" + childHelp
					}
				}
			}
		}
	} else {
		out += containerHelp
		if currentDepth < maxDepth {
			for _, child := range cc.Children {
				childHelp := child.GenerateHelp("", maxDepth, currentDepth+1)
				if strings.TrimSpace(childHelp) != "" {
					out += "\n" + childHelp
				}
			}
		}
	}

	return out
}

// Returns just the containers help without any subcommands
func (cc *CommandContainer) ContainerHelp(depth int) string {
	aliasesStr := ""
	if len(cc.Aliases) > 0 {
		aliasesStr = " {" + strings.Join(cc.Aliases, ",") + "}"
	}

	fmtName := fmt.Sprintf("%%-%ds", 15-(depth*2))

	return fmt.Sprintf("%s"+fmtName+"=%-20s : %s", Indent(depth), cc.Name, aliasesStr, cc.Description)
}

func (cc *CommandContainer) CheckMatch(raw string, trigger *TriggerData) bool {
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

func (cc *CommandContainer) HandleCommand(raw string, trigger *TriggerData, ctx context.Context) error {
	split := strings.SplitN(raw, " ", 2)

	found := false
	if len(split) > 1 {
		for _, v := range cc.Children {
			if v.CheckMatch(split[1], trigger) {
				v.HandleCommand(split[1], trigger, ctx)
				found = true
				break
			}
		}

		if !found {
			if cc.NotFoundHandler != nil {
				cc.NotFoundHandler.HandleCommand(split[1], trigger, ctx)
			} else {
				cc.SendUnknownHelp(trigger.Message, trigger.Session, split[1])
			}
		}
	} else {
		if cc.DefaultHandler != nil {
			cc.DefaultHandler.HandleCommand("", trigger, ctx)
		} else {
			cc.SendUnknownHelp(trigger.Message, trigger.Session, "")
		}
	}
	return nil
}

func (cc *CommandContainer) SendUnknownHelp(m *discordgo.Message, s *discordgo.Session, badCmd string) {
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s: Unknown subcommand (%q) D: see help for usage.", cc.Name, badCmd))
}
