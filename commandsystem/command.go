package commandsystem

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"strconv"
	"strings"
)

// A single command definition
type CommandDef struct {
	Name    string
	Aliases []string

	Description string

	RequiredArgs int
	Arguments    []*ArgumentDef

	RunFunc      func(cmd *ParsedCommand, m *discordgo.MessageCreate)
	HideFromHelp bool
}

func (c *CommandDef) String() string {
	aliasesString := ""

	if len(c.Aliases) > 0 {
		for k, v := range c.Aliases {
			if k != 0 {
				aliasesString += "/"
			}
			aliasesString += v
		}
		aliasesString = "(" + aliasesString + ")"
	}

	out := fmt.Sprintf("**%s**%s: %s.", c.Name, aliasesString, c.Description)
	if len(c.Arguments) > 0 {
		for _, v := range c.Arguments {
			out += fmt.Sprintf("\n     \\* %s - %s", v.String(), v.Description)
		}
	}
	return out
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
	Cmd  *CommandDef
	Args []*ParsedArgument
}

var (
	ErrIncorrectNumArgs    = errors.New("Icorrect number of arguments")
	ErrDiscordUserNotFound = errors.New("Discord user not found")
)

func ParseCommand(commandStr string, target *CommandDef, m *discordgo.MessageCreate, s *discordgo.Session) (*ParsedCommand, error) {
	// No arguments passed
	if len(target.Arguments) < 1 {
		return &ParsedCommand{
			Name: target.Name,
			Cmd:  target,
		}, nil
	}

	parsedArgs := make([]*ParsedArgument, len(target.Arguments))

	// Filter out command from string
	buf := strings.Replace(commandStr, target.Name, "", 1)
	buf = strings.TrimSpace(buf)
	curIndex := 0

	for k, v := range target.Arguments {
		var val interface{}
		var next int
		var err error
		switch v.Type {
		case ArgumentTypeNumber:
			val, next, err = ParseNumber(buf[curIndex:])
		case ArgumentTypeString:
			val, next, err = ParseString(buf[curIndex:])
		case ArgumentTypeUser:
			val, next, err = ParseUser(buf[curIndex:], m.Message, s)
		}
		raw := buf[curIndex : curIndex+next]

		curIndex += next
		curIndex += TrimSpaces(buf[curIndex:])

		parsedArgs[k] = &ParsedArgument{
			Raw:    raw,
			Parsed: val,
		}
		if err != nil {
			return nil, err
		}
		if curIndex >= len(buf) {
			if k < target.RequiredArgs {
				return nil, ErrIncorrectNumArgs
			}
			break
		}
	}

	return &ParsedCommand{
		Name: target.Name,
		Cmd:  target,
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
