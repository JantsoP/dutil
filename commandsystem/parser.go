package commandsystem

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"strconv"
	"strings"
)

// Parses a command into a ParsedCommand
// Arguments are split at space or you can put arguments inside quotes
// You can escape both space and quotes using '\"' or '\ ' ('\\' to escape the escaping)
// Quotes in the middle of an argument is trated as a normal character and not a seperator
func (sc *Command) ParseCommand(raw string, m *discordgo.MessageCreate, s *discordgo.Session) (*ExecData, error) {

	data := &ExecData{
		Session: s,
		Message: m.Message,
		Command: sc,
	}

	// Retrieve guild and channel if possible (session not provided in testing)
	var channel *discordgo.Channel
	var guild *discordgo.Guild
	if s != nil {
		var err error
		channel, err = s.State.Channel(m.ChannelID)
		if err != nil {
			return nil, err
		}
		data.Channel = channel

		guild, err = s.State.Guild(channel.ID)
		if err == nil {
			data.Guild = guild
		}

		s.State.RLock()
		defer s.State.RUnlock()
	}

	// No arguments needed
	if len(sc.Arguments) < 1 {
		return data, nil
	}

	// Strip away the command name (or alias if that was what triggered it)
	buf := ""
	if sc.Name != "" {
		split := strings.SplitN(raw, " ", 2)
		if len(split) < 1 {
			return nil, errors.New("Command not specified")
		}

		if strings.EqualFold(split[0], strings.ToLower(sc.Name)) {
			buf = raw[len(strings.ToLower(sc.Name)):]
		} else {
			for _, alias := range sc.Aliases {
				if strings.EqualFold(alias, split[0]) {
					buf = raw[len(strings.ToLower(alias)):]
					break
				}
			}
		}
	}

	buf = strings.TrimSpace(buf)
	parsedArgs := make([]*ParsedArgument, len(sc.Arguments))
	for i, v := range sc.Arguments {
		if v.Default != nil {
			parsedArgs[i] = &ParsedArgument{Parsed: v.Default}
		}
	}

	data.Args = parsedArgs

	// No parameters provided, and none required, just handle the mofo
	if buf == "" {
		if sc.RequiredArgs == 0 && len(sc.ArgumentCombos) < 1 {
			return data, nil
		} else {
			if len(sc.ArgumentCombos) < 1 {
				err := sc.ErrMissingArgs(0)
				return nil, err
			} else {
				// Check if one of the combox accepts zero arguments
				for _, combo := range sc.ArgumentCombos {
					if len(combo) < 1 {
						return data, nil
					}
				}
			}

			return nil, ErrInvalidParameters
		}
	}

	rawArgs := ReadArgs(buf)
	selectedCombo, ok := sc.findCombo(rawArgs)
	if !ok {
		if len(sc.ArgumentCombos) < 1 {
			err := sc.ErrMissingArgs(len(rawArgs))
			return nil, err
		}
		return nil, ErrInvalidParameters
	}

	// Parse the arguments and fill up the PArsedArgs slice
	for k, comboArg := range selectedCombo {
		var val interface{}
		var err error

		buf := rawArgs[k].Raw.Str
		// If last arg att all the remaning rawargs, building up the
		if k == len(selectedCombo)-1 {
			for i := k + 1; i < len(rawArgs); i++ {
				switch rawArgs[i].Raw.Seperator {
				case ArgSeperatorSpace:
					buf += " " + rawArgs[i].Raw.Str
				case ArgSeperatorQuote:
					buf += " \"" + rawArgs[i].Raw.Str + "\""
				}
			}
		}

		switch sc.Arguments[comboArg].Type {
		case ArgumentString:
			val = buf
		case ArgumentNumber:
			val, err = ParseNumber(buf)
		case ArgumentUser:
			if channel == nil || channel.IsPrivate {
				continue // can't provide users in direct messages
			}
			val, err = ParseUser(buf, m.Message, guild)
		}

		if err != nil {
			return nil, errors.New("Failed parsing arguments: " + err.Error())
		}

		parsedArgs[comboArg] = &ParsedArgument{
			Raw:    buf,
			Parsed: val,
		}
	}

	return data, nil
}

// Finds a proper argument combo from the provided args
func (sc *Command) findCombo(rawArgs []*MatchedArg) ([]int, bool) {
	// Find a argument combo to match against
	if len(sc.ArgumentCombos) < 1 {
		if sc.RequiredArgs > 0 && len(rawArgs) < sc.RequiredArgs {
			return nil, false
		}

		size := len(rawArgs)
		if size > len(sc.Arguments) {
			size = len(sc.Arguments)
		}

		selectedCombo := make([]int, size)
		for i, _ := range rawArgs {
			if i >= len(sc.Arguments) {
				break
			}

			selectedCombo[i] = i
		}
		return selectedCombo, true
	}

	var selectedCombo []int
	var ok bool

	// Find a possible match
OUTER:
	for _, combo := range sc.ArgumentCombos {
		if len(combo) > len(rawArgs) {
			// No match
			continue
		}

		// See if this combos arguments matches that of the parsed command
		for k, comboArg := range combo {
			arg := sc.Arguments[comboArg]

			if !sc.checkArgumentMatch(rawArgs[k], arg.Type) {
				continue OUTER // No match
			}
		}

		// We got a match, if this match is stronger than the last one set it as selected
		if len(combo) > len(selectedCombo) || !ok {
			selectedCombo = combo
			ok = true
		}
	}

	return selectedCombo, ok
}

func (sc *Command) ErrMissingArgs(provided int) error {
	names := ""
	for i, v := range sc.Arguments {
		if i < provided {
			continue
		}

		if i != provided {
			names += ", "
		}

		if i > sc.RequiredArgs {
			names += "(optional)"
		}
		names += v.Name

	}

	return fmt.Errorf("Missing arguments: %s.", names)
}

func (sc *Command) checkArgumentMatch(raw *MatchedArg, definition ArgumentType) bool {
	switch definition {
	case ArgumentNumber:
		return raw.Type == ArgumentNumber
	case ArgumentUser:
		// Check if a user mention is required
		// Otherwise it can be of any type
		if sc.UserArgRequireMention {
			return raw.Type == ArgumentUser
		} else {
			return true
		}
	case ArgumentString:
		// Both number and user can be a string
		// So it willl always match string no matter what
		return true
	}

	return false
}

func TrimSpaces(buf string) (index int) {
	for k, v := range buf {
		if v != ' ' {
			return k
		}
	}
	return len(buf)
}

type MatchedArg struct {
	Type ArgumentType
	Raw  *RawArg
}

type ArgSeperator int

const (
	ArgSeperatorSpace ArgSeperator = iota
	ArgSeperatorQuote
)

type RawArg struct {
	Str       string
	Seperator ArgSeperator
}

// Reads the command line and seperates it into a slice of strings
// These strings are later processed depending on the argument type they belong to
func ReadArgs(in string) []*MatchedArg {
	rawArgs := make([]*RawArg, 0)

	curBuf := ""
	escape := false
	quoted := false
	for _, r := range in {
		// Apply or remove escape mode
		if r == '\\' {
			if escape {
				escape = false
				curBuf += "\\"
			} else {
				escape = true
			}

			continue
		}

		// Check for other special tokens
		isSpecialToken := false
		if !escape {
			isSpecialToken = true
			switch r {
			case ' ': // Split the args here if it's not quoted
				if curBuf != "" && !quoted {
					rawArgs = append(rawArgs, &RawArg{curBuf, ArgSeperatorSpace})
					curBuf = ""
					quoted = false
				} else if quoted { // If it is quoted proceed as it was a normal rune
					isSpecialToken = false
				}
			case '"':
				// Set quoted mode if at start of arg, split arg if already in quoted mode
				// treat quotes in the middle of arg as normal
				if curBuf == "" && !quoted {
					quoted = true
				} else if quoted {
					rawArgs = append(rawArgs, &RawArg{curBuf, ArgSeperatorQuote})
					curBuf = ""
					quoted = false
				} else {
					isSpecialToken = false
				}
			default:
				isSpecialToken = false
			}
		}

		if !isSpecialToken {
			curBuf += string(r)
		}

		// Reset escape mode
		escape = false
	}

	// Something was left in the buffer just add it to the end
	if curBuf != "" {
		rawArgs = append(rawArgs, &RawArg{curBuf, ArgSeperatorSpace})
	}

	// Match up the arguments to possible datatypes
	// Used when finding the proper combo
	// Only distinguishes between numbers, strings amnd user mentions atm
	// Which means it won't work properly if you have 2 combos
	// where the only differences are string and user
	// it will not work as expected
	out := make([]*MatchedArg, len(rawArgs))
	for i, raw := range rawArgs {
		// Check for number
		_, err := strconv.ParseFloat(raw.Str, 64)
		if err == nil {
			out[i] = &MatchedArg{Type: ArgumentNumber, Raw: raw}
			continue
		}
		if strings.Index(raw.Str, "<@") == 0 {
			if raw.Str[len(raw.Str)-1] == '>' {
				// Mention, so user
				out[i] = &MatchedArg{Type: ArgumentUser, Raw: raw}
				continue
			}
		}
		// Else it could be anything, no definitive answer
		out[i] = &MatchedArg{Type: ArgumentString, Raw: raw}
	}

	return out
}

// Parses a discord user from buf and returns the error if any
func ParseUser(buf string, m *discordgo.Message, guild *discordgo.Guild) (user *discordgo.User, err error) {
	field := buf
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
		user, err = FindDiscordUser(field, m, guild)
	}

	if user == nil {
		err = ErrDiscordUserNotFound
	}

	return
}

// Parses a number from buf and returns the end index and error if any
func ParseNumber(buf string) (num float64, err error) {
	num, err = strconv.ParseFloat(buf, 64)
	return
}

var ErrNotLoggedIn = errors.New("Not logged into discord")

func FindDiscordUser(str string, m *discordgo.Message, guild *discordgo.Guild) (*discordgo.User, error) {
	if guild == nil {
		return nil, ErrDiscordUserNotFound
	}

	for _, v := range guild.Members {
		if strings.EqualFold(str, v.User.Username) {
			return v.User, nil
		}
	}

	return nil, ErrDiscordUserNotFound
}
