package commandsystem

import (
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dutil"
	"log"
	"runtime/debug"
	"strings"
)

type CommandSystem struct {
	Commands              []*CommandDef // Registered commands
	DefaultMentionHandler *CommandDef   // Called when no other handler is found

	SendStackOnPanic bool // Dumps the stack in a chat message when a panic happens in a command handler
	Session          *discordgo.Session
}

func (cs *CommandSystem) RegisterCommands(cmds ...*CommandDef) {
	cs.Commands = append(cs.Commands, cmds...)
}

func (cs *CommandSystem) HandleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author != nil && m.Author.Bot {
		return // Ignore bots
	}

	// Catch panics so that panics in command handlers does not stop the bot
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			log.Println("Recovered from panic ", r, "\n", m.Content, "\n", stack)
			if cs.SendStackOnPanic {
				_, err := dutil.SplitSendMessage(s, m.ChannelID, "Panic when handling Command!! ```\n"+stack+"\n```")
				if err != nil {
					log.Println("Failed sending stacktrace", err)
				}
			}
		}
	}()

	if s.State == nil || s.State.User == nil {
		return // Can't handle message if we don't know our id
	}

	commandStr := ""

	// Check for mention
	id := s.State.User.ID
	if strings.Index(m.Content, "<@"+id+">") == 0 {
		commandStr = strings.Replace(m.Content, "<@"+id+">", "", 1)
	} else if strings.Index(m.Content, "<@!"+id+">") == 0 {
		commandStr = strings.Replace(m.Content, "<@!"+id+">", "", 1)
	} else {
		return
	}

	commandStr = strings.TrimSpace(commandStr)

	// Check if any additional fields were provided to the command, if not just run the default command if possible
	fields := strings.Fields(commandStr)
	if len(fields) < 1 {
		if cs.DefaultMentionHandler != nil && cs.DefaultMentionHandler.RequiredArgs == 0 {
			cs.DefaultMentionHandler.RunFunc(&ParsedCommand{Args: make([]*ParsedArgument, len(cs.DefaultMentionHandler.Arguments))}, m)
		}
		return
	}

	// Find a hadnler by name
	cmdName := strings.ToLower(fields[0])
	for _, v := range cs.Commands {
		match := v.Name == cmdName
		if !match {
			for _, alias := range v.Aliases {
				if alias == cmdName {
					match = true
					break
				}
			}
		}

		if match {
			parsed, err := ParseCommand(commandStr, v, m, s)
			if err != nil {
				dutil.SplitSendMessage(s, m.ChannelID, "Error parsing command: "+err.Error())
				return
			}
			if v.RunFunc != nil {
				v.RunFunc(parsed, m)
			}
			return
		}
	}

	// No handler found, check the default one
	if cs.DefaultMentionHandler != nil {
		parsed, err := ParseCommand(commandStr, cs.DefaultMentionHandler, m, s)
		if err != nil {
			dutil.SplitSendMessage(s, m.ChannelID, "Error parsing command: "+err.Error())
			return
		}
		if cs.DefaultMentionHandler.RunFunc != nil {
			cs.DefaultMentionHandler.RunFunc(parsed, m)
		}
	}
}

func (cs *CommandSystem) GenerateHelp(byCmd string) string {
	out := ""
	for _, cmd := range cs.Commands {
		if cmd.HideFromHelp {
			continue
		}

		if byCmd == "" || byCmd == cmd.Name {
			out += " - " + cmd.String() + "\n"
		}
	}
	return out
}
