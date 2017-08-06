package commandsystem

import (
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dutil"
	"log"
	"runtime/debug"
	"strings"
)

type Source int

const (
	SourceMention Source = iota // Command triggered by mention
	SourcePrefix                // Command triggered by prefix
	SourceDM                    // Command in a direct message
	SourceHelp                  // Triggered by help, to check if its matched if a specific command was asked for
)

type System struct {
	Commands []CommandHandler // Registered commands

	DefaultMentionHandler CommandHandler // Called when no other handler is found and the bot is mentioned
	DefaultDMHandler      CommandHandler // Called when no other handler is found this is a dm channel
	DefaultHandler        CommandHandler // Called when no other handler is found and the bot is not mentioned
	Prefix                PrefixProvider // Alternative command prefix

	// If set, called to censor the error output, (such as tokens and whatnot)
	// If not set only the discord auth token will be censored
	CensorError func(err error) string

	IgnoreBots       bool // Set to ignore bots (NewSystem sets it to true)
	SendStackOnPanic bool // Dumps the stack in a chat message when a panic happens in a command handler
	SendError        bool // Set to send error messages that a command handler returned
}

// Returns a system with default configuration
// Will add messagecreate handler if session is not nil
// If prefix is not zero ("") it will also add a SimplePrefixProvider
func NewSystem(session *discordgo.Session, prefix string) *System {
	cs := &System{
		Commands:   make([]CommandHandler, 0),
		IgnoreBots: true,
	}

	if session != nil {
		session.AddHandler(cs.HandleMessageCreate)
	}

	if prefix != "" {
		cs.Prefix = NewSimplePrefixProvider(prefix)
	}
	return cs
}

func (cs *System) RegisterCommands(cmds ...CommandHandler) {
	cs.Commands = append(cs.Commands, cmds...)
}

func (cs *System) HandleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author != nil && m.Author.Bot {
		return // Ignore bots
	}

	if s.State == nil || s.State.User == nil {
		return // Can't handle message if we don't know our id
	}

	didMatch := false

	// Catch panics so that panics in command handlers does not stop the bot
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			log.Println("[CommandSystem]: Recovered from panic in CommandHandler:", r, "\n", m.Content, "\n", stack)
			if didMatch {
				if cs.SendStackOnPanic {
					_, err := dutil.SplitSendMessage(s, m.ChannelID, "Panic when handling Command! ```\n"+stack+"\n```")
					if err != nil {
						log.Println("[CommandSystem]: Failed sending stacktrace", err)
					}
				} else {
					s.ChannelMessageSend(m.ChannelID, "Bot is panicking! Contact the bot owner!")
				}
			}
		}
	}()

	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		log.Println("[CommandSystem]: Failed getting channel from state:", err)
		return // Need channel to function
	}

	// Check if mention or prefix matches
	commandStr, mention, ok := cs.CheckPrefix(channel, s, m)

	// No prefix found :'(
	if !ok {
		return
	}

	var source Source
	if mention {
		source = SourceMention
	} else if channel.Type == discordgo.ChannelTypeDM {
		source = SourceDM
	} else {
		source = SourcePrefix
	}

	// Check if any additional fields were provided to the command, if not just run the default command if possible
	if commandStr == "" {

		didMatch = true
		cs.triggerDefaultHandler(commandStr, source, m, s)

		return
	}

	// Find a handler
	for _, v := range cs.Commands {
		if v.CheckMatch(commandStr, source, m, s) {
			didMatch = true
			_, err := v.HandleCommand(commandStr, source, m, s)
			cs.CheckCommandError(err, m.ChannelID, s)
			return
		}
	}

	didMatch = true
	// No handler found, check the default one
	cs.triggerDefaultHandler(commandStr, source, m, s)

}

// Trigger the default handler for the appropiate source
func (cs *System) triggerDefaultHandler(cmdStr string, source Source, m *discordgo.MessageCreate, s *discordgo.Session) {

	var err error

	switch source {
	case SourceDM:
		if cs.DefaultDMHandler != nil {
			_, err = cs.DefaultDMHandler.HandleCommand(cmdStr, source, m, s)
		}
	case SourceMention:
		if cs.DefaultMentionHandler != nil {
			_, err = cs.DefaultMentionHandler.HandleCommand(cmdStr, source, m, s)
		}
	default:
		if cs.DefaultHandler != nil {
			_, err = cs.DefaultHandler.HandleCommand(cmdStr, source, m, s)
		}
	}

	cs.CheckCommandError(err, m.ChannelID, s)
}

func (cs *System) CheckPrefix(channel *discordgo.Channel, s *discordgo.Session, m *discordgo.MessageCreate) (cmdStr string, mention bool, ok bool) {

	// DM Handlers require no prefix
	if channel.Type == discordgo.ChannelTypeDM {
		return m.Content, false, true
	}

	// Check for mention
	id := s.State.User.ID
	if strings.Index(m.Content, "<@"+id+">") == 0 { // Normal mention
		ok = true
		mention = true
		cmdStr = strings.Replace(m.Content, "<@"+id+">", "", 1)
	} else if strings.Index(m.Content, "<@!"+id+">") == 0 { // Nickname mention
		ok = true
		mention = true
		cmdStr = strings.Replace(m.Content, "<@!"+id+">", "", 1)
	}

	if ok {
		cmdStr = strings.TrimSpace(cmdStr)
		return
	}

	// Check for custom prefix
	if cs.Prefix == nil {
		return
	}

	prefix := cs.Prefix.GetPrefix(s, m)
	if prefix == "" {
		return // ...
	}

	if strings.Index(m.Content, prefix) == 0 {
		ok = true
		cmdStr = strings.Replace(m.Content, prefix, "", 1)
	}
	return
}

// Generates help for all commands
// Will probably be reworked at one point
func (cs *System) GenerateHelp(target string, depth int) string {
	out := ""
	for _, cmd := range cs.Commands {
		help := cmd.GenerateHelp(target, depth, 0)
		if help != "" {
			out += help + "\n"
		}
	}

	// No commands
	if out == "" {
		return ""
	}

	return "```ini\n" + out + "```"
}

// Checks the error output of a command and handles it as appropiate
func (cs *System) CheckCommandError(err error, channel string, s *discordgo.Session) {
	if err != nil {
		if cs.SendError {
			msg := "Command Error"
			if cs.CensorError != nil {
				msg += ": " + cs.CensorError(err)
			} else {
				msg += ": " + strings.Replace(err.Error(), s.Token, "<censored token>", -1)
			}
			dutil.SplitSendMessage(s, channel, msg)
		}
		log.Output(2, "Error handling command: "+err.Error())
	}
}

// Retrieves the prefix that might be different on a per server basis
type PrefixProvider interface {
	GetPrefix(s *discordgo.Session, m *discordgo.MessageCreate) string
}

// Simple Prefix provider for global fixed prefixes
type SimplePrefixProvider struct {
	Prefix string
}

func NewSimplePrefixProvider(prefix string) PrefixProvider {
	return &SimplePrefixProvider{Prefix: prefix}
}

func (pp *SimplePrefixProvider) GetPrefix(s *discordgo.Session, m *discordgo.MessageCreate) string {
	return pp.Prefix
}

func Indent(depth int) string {
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "__"
	}
	return indent
}
