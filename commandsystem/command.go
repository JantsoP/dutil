package commandsystem

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"strconv"
	"strings"
)

type CommandHandlerFunc func(raw string, m *discordgo.MessageCreate, s *discordgo.Session)

type CommandHandler interface {
	CheckMatch(raw string, m *discordgo.MessageCreate, s *discordgo.Session) bool
	HandleCommand(raw string, m *discordgo.MessageCreate, s *discordgo.Session) error
	GenerateHelp(target string, depth int) string // Generates help
}

// A single command definition
type SimpleCommand struct {
	Name        string   // Name of command, what its called from
	Aliases     []string // Aliases which it can also be called from
	Description string

	HideFromHelp            bool // Hide it from help
	IgnoreUserNotFoundError bool // Instead of throwing a User not found error, it will ignore it if it's not a requireed argument
	RunInDm                 bool // Run in dms, users can't be provided as arguments then

	RequiredArgs int // Number of reuquired arguments
	Arguments    []*ArgumentDef

	RunFunc func(cmd *ParsedCommand, m *discordgo.MessageCreate) error
}

func (sc *SimpleCommand) GenerateHelp(target string, depth int) string {
	if target != "" {
		if !sc.CheckMatch(target, nil, nil) {
			return ""
		}
	}

	if sc.HideFromHelp {
		return ""
	}

	aliasesString := ""

	if len(sc.Aliases) > 0 {
		for k, v := range sc.Aliases {
			if k != 0 {
				aliasesString += "/"
			}
			aliasesString += v
		}
		aliasesString = "(" + aliasesString + ")"
	}

	out := fmt.Sprintf(" - **%s**%s: %s.", sc.Name, aliasesString, sc.Description)
	if len(sc.Arguments) > 0 {
		for _, v := range sc.Arguments {
			out += fmt.Sprintf("\n  \\* %s - %s", v.String(), v.Description)
		}
	}
	return out
}

func (sc *SimpleCommand) CheckMatch(raw string, m *discordgo.MessageCreate, s *discordgo.Session) bool {
	fields := strings.SplitN(raw, " ", 2)
	if len(fields) < 1 {
		return false
	}

	match := strings.EqualFold(fields[0], sc.Name)
	if !match {
		for _, v := range sc.Aliases {
			if strings.EqualFold(fields[0], v) {
				match = true
				break
			}
		}
	}

	if !match {
		return false
	}

	// We don't need channel info if it runs in all cases anyways
	if sc.RunInDm {
		return true
	}

	if s == nil {
		return true
	}

	// Does not run in dm's so check if this is a dm
	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		return false
	}

	if !channel.IsPrivate {
		return true
	}

	return false
}

func (sc *SimpleCommand) HandleCommand(raw string, m *discordgo.MessageCreate, s *discordgo.Session) error {
	parsed, err := sc.ParseCommand(raw, m, s)
	if err != nil {
		return err
	}

	if sc.RunFunc != nil {
		return sc.RunFunc(parsed, m)
	}

	return nil
}

func (sc *SimpleCommand) ParseCommand(raw string, m *discordgo.MessageCreate, s *discordgo.Session) (*ParsedCommand, error) {
	// No arguments needed
	if len(sc.Arguments) < 1 {
		return &ParsedCommand{
			Name: sc.Name,
			Cmd:  sc,
		}, nil
	}

	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		return nil, err
	}

	parsedArgs := make([]*ParsedArgument, len(sc.Arguments))

	buf := ""
	if sc.Name != "" {
		split := strings.SplitN(raw, " ", 2)
		if len(split) < 1 {
			return nil, errors.New("Command not specified")
		}

		if split[0] == strings.ToLower(sc.Name) {
			buf = raw[len(strings.ToLower(sc.Name)):]
		} else {
			for _, alias := range sc.Aliases {
				if strings.ToLower(alias) == strings.ToLower(split[0]) {
					buf = raw[len(strings.ToLower(alias)):]
					break
				}
			}
		}
	}

	buf = strings.TrimSpace(buf)
	curIndex := 0

	if buf == "" {
		if sc.RequiredArgs == 0 {
			return &ParsedCommand{
				Name: sc.Name,
				Cmd:  sc,
				Args: parsedArgs,
			}, nil
		} else {
			return nil, errors.New("Not enough argument provided, see help for more info")
		}
	}

OUTER:
	for k, v := range sc.Arguments {
		var val interface{}
		var next int
		var err error
		switch v.Type {
		case ArgumentTypeNumber:
			val, next, err = ParseNumber(buf[curIndex:])
		case ArgumentTypeString:
			val, next, err = ParseString(buf[curIndex:])
		case ArgumentTypeUser:
			if channel.IsPrivate {
				break OUTER
			}
			val, next, err = ParseUser(buf[curIndex:], m.Message, s)
		}
		rawArg := buf[curIndex : curIndex+next]

		curIndex += next
		curIndex += TrimSpaces(buf[curIndex:])

		parsedArgs[k] = &ParsedArgument{
			Raw:    rawArg,
			Parsed: val,
		}
		if err != nil {
			if err == ErrDiscordUserNotFound && sc.IgnoreUserNotFoundError {
				parsedArgs[k] = nil
			} else {
				return nil, err
			}
		}
		if curIndex >= len(buf) {
			if k < sc.RequiredArgs {
				return nil, ErrIncorrectNumArgs
			}
			break
		}
	}

	return &ParsedCommand{
		Name: sc.Name,
		Cmd:  sc,
		Args: parsedArgs,
	}, nil
}

func TrimSpaces(buf string) (index int) {
	for k, v := range buf {
		if v != ' ' {
			return k
		}
	}
	return len(buf)
}

// Parses a discord user from buf and returns the end index and error if any
func ParseUser(buf string, m *discordgo.Message, s *discordgo.Session) (user *discordgo.User, index int, err error) {
	nextSpace := findNextSpace(buf)

	field := buf[:nextSpace]
	index = nextSpace

	if strings.Index(buf, "<@") == 0 {
		// Direct mention
		id := field[2 : len(field)-1]
		if id[0] == '!' {
			// Nickname mention
			id = id[1:]
		}

		for _, v := range m.Mentions {
			if id == v.ID {
				user = v
				break
			}
		}
	} else {
		// Search for username
		user, err = FindDiscordUser(field, m, s)
	}

	if user == nil {
		err = ErrDiscordUserNotFound
	}

	return
}

func ParseString(buf string) (s string, index int, err error) {
	nextSpace := findNextSpace(buf)
	index = nextSpace
	s = buf[:nextSpace]
	return
}

// Parses a number from buf and returns the end index and error if any
func ParseNumber(buf string) (num float64, index int, err error) {
	nextSpace := findNextSpace(buf)
	index = nextSpace
	num, err = strconv.ParseFloat(buf[:nextSpace], 64)
	return
}

func findNextSpace(buf string) int {
	nextSpace := strings.Index(buf, " ")
	if nextSpace == -1 {
		nextSpace = len(buf)
	}
	return nextSpace
}

var ErrNotLoggedIn = errors.New("Not logged into discord")

func FindDiscordUser(str string, m *discordgo.Message, s *discordgo.Session) (*discordgo.User, error) {
	if s == nil || s.Token == "" {
		return nil, ErrNotLoggedIn
	}

	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		return nil, err
	}

	guild, err := s.State.Guild(channel.GuildID)
	if err != nil {
		return nil, err
	}

	s.State.RLock()
	defer s.State.RUnlock()
	for _, v := range guild.Members {
		if strings.EqualFold(str, v.User.Username) {
			return v.User, nil
		}
	}

	return nil, ErrDiscordUserNotFound
}

type ArgumentType int

const (
	ArgumentTypeString ArgumentType = iota
	ArgumentTypeNumber
	ArgumentTypeUser
)

func (a ArgumentType) String() string {
	switch a {
	case ArgumentTypeString:
		return "String"
	case ArgumentTypeNumber:
		return "Number"
	case ArgumentTypeUser:
		return "@User"
	}
	return "???"
}

type ArgumentDef struct {
	Name        string
	Description string
	Type        ArgumentType
}

func (a *ArgumentDef) String() string {

	return a.Name + ":" + a.Type.String() + ""
}

type ParsedArgument struct {
	Raw    string
	Parsed interface{}
}

func (p *ParsedArgument) Int() int {
	val, _ := p.Parsed.(float64)
	return int(val)
}

func (p *ParsedArgument) Str() string {
	val, _ := p.Parsed.(string)
	return val
}

func (p *ParsedArgument) Float() float64 {
	val, _ := p.Parsed.(float64)
	return val
}

func (p *ParsedArgument) DiscordUser() *discordgo.User {
	val, _ := p.Parsed.(*discordgo.User)
	return val
}

type ParsedCommand struct {
	Name string
	Cmd  *SimpleCommand
	Args []*ParsedArgument
}

var (
	ErrIncorrectNumArgs    = errors.New("Icorrect number of arguments")
	ErrDiscordUserNotFound = errors.New("Discord user not found")
)
