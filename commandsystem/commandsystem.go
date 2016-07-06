package commandsystem

import (
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dutil"
	"log"
	"runtime/debug"
	"strings"
)

type System struct {
	Commands       []CommandHandler // Registered commands
	DefaultHandler CommandHandler   // Called when no other handler is found

	SendStackOnPanic bool // Dumps the stack in a chat message when a panic happens in a command handler
	Session          *discordgo.Session
}

func (cs *System) RegisterCommands(cmds ...CommandHandler) {
	cs.Commands = append(cs.Commands, cmds...)
}

func (cs *System) HandleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
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

	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		log.Println("Failed getting channel from state:", err)
		return // Need channel to function
	}

	commandStr := ""
	// Check for mention
	if channel.IsPrivate {
		commandStr = m.Content
	} else {
		id := s.State.User.ID
		if strings.Index(m.Content, "<@"+id+">") == 0 { // Normal mention
			commandStr = strings.Replace(m.Content, "<@"+id+">", "", 1)
		} else if strings.Index(m.Content, "<@!"+id+">") == 0 { // Nickname mention
			commandStr = strings.Replace(m.Content, "<@!"+id+">", "", 1)
		} else {
			return
		}
	}

	commandStr = strings.TrimSpace(commandStr)

	// Check if any additional fields were provided to the command, if not just run the default command if possible
	if commandStr == "" {
		if cs.DefaultHandler != nil {
			err := cs.DefaultHandler.HandleCommand(commandStr, m, s)
			cs.CheckCommandError(err, m.ChannelID)
		}
		return
	}

	// Find a handler
	for _, v := range cs.Commands {
		if v.CheckMatch(commandStr, m, s) {
			err := v.HandleCommand(commandStr, m, s)
			cs.CheckCommandError(err, m.ChannelID)
			return
		}
	}

	// No handler found, check the default one
	if cs.DefaultHandler != nil {
		err := cs.DefaultHandler.HandleCommand("", m, s)
		cs.CheckCommandError(err, m.ChannelID)
	}
}

func (cs *System) GenerateHelp(target string, depth int) string {
	out := ""
	for _, cmd := range cs.Commands {
		help := cmd.GenerateHelp(target, depth)
		if help != "" {
			out += help + "\n"
		}
	}
	return out
}

func (cs *System) CheckCommandError(err error, channel string) {
	if err != nil {
		log.Println("Error handling command:", err)
		dutil.SplitSendMessage(cs.Session, channel, "Error handling command: "+err.Error())
	}
}
