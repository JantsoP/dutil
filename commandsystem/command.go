package commandsystem

import (
	"errors"
	"fmt"
	"github.com/jonas747/discordgo"
	"strconv"
	"strings"
)

var (
	// Returned if the parameters passed to the command didnt match the command definition
	ErrInvalidParameters   = errors.New("Invalid parameters passed to command, see help for usage")
	ErrDiscordUserNotFound = errors.New("Discord user not found")
)

type CommandHandlerFunc func(raw string, m *discordgo.MessageCreate, s *discordgo.Session)

// Represents a command handler to handle commands
type CommandHandler interface {
	// Called to check if the command matched "raw"
	CheckMatch(raw string, source CommandSource, m *discordgo.MessageCreate, s *discordgo.Session) bool

	// Handle the command itself
	HandleCommand(raw string, source CommandSource, m *discordgo.MessageCreate, s *discordgo.Session) error

	// Generates help output, maxDepth is how far into container help will go
	GenerateHelp(target string, maxDepth, currentDepth int) string
}

// A general purpose CommandHandler implementation
// With support for aliases, automatically parsed arguments
// with different combos and optionally ran in dm
//
// LIMITATIONS TO ARGUMENT COMBOS:
// They need a length difference or one of the differences need to be a number
// What works:
// [string, string] : [string]
// [string, number] : [number, string]
//
// For the below to work you need to have "UserArgRequireMention" set
// otherwise it won't be able to distinguish between them:
// [string, user] : [user, string]
// You can't do:
// [string, string] : [string, string] <- no way to determine what combo is the correct one
type SimpleCommand struct {
	Name        string   // Name of command, what its called from
	Aliases     []string // Aliases which it can also be called from
	Description string

	HideFromHelp            bool // Hide it from help
	UserArgRequireMention   bool // Set to require user mention in user mentions, otherwise it will attempt to search
	IgnoreUserNotFoundError bool // Instead of throwing a User not found error, it will ignore it if it's not a required argument

	RunInDm        bool // Run in dms, users can't be provided as arguments then
	IgnoreMentions bool // Will not be triggered by mentions

	Arguments      []*ArgumentDef // Slice of argument definitions, ParsedCommand.Args will always be the same size as this slice (although the data may be nil)
	RequiredArgs   int            // Number of reuquired arguments, ignored if combos is specified
	ArgumentCombos [][]int        // Slice of argument pairs, will override RequiredArgs if specified

	RunFunc func(cmd *ParsedCommand, m *discordgo.MessageCreate) error
}

func (sc *SimpleCommand) GenerateHelp(target string, maxDepth, currentDepth int) string {
	if target != "" {
		if !sc.CheckMatch(target, CommandSourceHelp, nil, nil) {
			return ""
		}
	}

	if sc.HideFromHelp {
		return ""
	}

	// Generate aliases
	aliasesString := ""
	if len(sc.Aliases) > 0 {
		for k, v := range sc.Aliases {
			if k != 0 {
				aliasesString += "/"
			}
			aliasesString += v
		}
		aliasesString = " {" + aliasesString + "}"
	}

	// Generate arguments
	argsString := ""
	for k, arg := range sc.Arguments {
		if k < sc.RequiredArgs {
			argsString += fmt.Sprintf(" <%s>", arg.String())
		} else {
			argsString += fmt.Sprintf(" (%s)", arg.String())
		}
	}

	middle := aliasesString + argsString

	// Final format
	fmtName := fmt.Sprintf("%%-%ds", 15-(currentDepth*2))

	out := fmt.Sprintf("%s"+fmtName+"=%-20s : %s", Indent(currentDepth), sc.Name, middle, sc.Description)

	return out
}

func (sc *SimpleCommand) CheckMatch(raw string, source CommandSource, m *discordgo.MessageCreate, s *discordgo.Session) bool {
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

	// Check if this is a mention and ignore if so
	if source == CommandSourceMention && sc.IgnoreMentions {
		return false
	}

	// Same as above with dm's
	if source == CommandSourceDM && !sc.RunInDm {
		return false
	}

	return true
}

func (sc *SimpleCommand) HandleCommand(raw string, source CommandSource, m *discordgo.MessageCreate, s *discordgo.Session) error {
	parsed, err := sc.ParseCommand(raw, m, s)
	if err != nil {
		return err
	}
	parsed.Source = source

	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		return err
	}

	guild, err := s.State.Guild(channel.GuildID)
	if err != nil {
		return err
	}
	parsed.Channel = channel
	parsed.Guild = guild

	if sc.RunFunc != nil {
		return sc.RunFunc(parsed, m)
	}

	return nil
}

// Parses a command into a ParsedCommand
// Arguments are split at space or you can put arguments inside quotes
// You can escape both space and quotes using '\"' or '\ ' ('\\' to escape the escaping)
// Quotes in the middle of an argument is trated as a normal character and not a seperator
func (sc *SimpleCommand) ParseCommand(raw string, m *discordgo.MessageCreate, s *discordgo.Session) (*ParsedCommand, error) {
	// No arguments needed
	if len(sc.Arguments) < 1 {
		return &ParsedCommand{
			Name: sc.Name,
			Cmd:  sc,
		}, nil
	}

	// Retrieve channel if possible (session not provided in testing)
	var channel *discordgo.Channel
	if s != nil {
		var err error
		channel, err = s.State.Channel(m.ChannelID)
		if err != nil {
			return nil, err
		}
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

	// No parameters provided, and none required, just handle the mofo
	if buf == "" {
		if sc.RequiredArgs == 0 && len(sc.ArgumentCombos) < 1 {
			return &ParsedCommand{
				Name: sc.Name,
				Cmd:  sc,
				Args: parsedArgs,
			}, nil
		} else {
			return nil, errors.New("Not enough arguments provided, see help for more info")
		}
	}

	rawArgs := ReadArgs(buf)
	selectedCombo, ok := sc.findCombo(rawArgs)
	if !ok {
		return nil, ErrInvalidParameters
	}

	// Parse the arguments and fill up the PArsedArgs slice
	for k, comboArg := range selectedCombo {
		var val interface{}
		var err error

		buf := rawArgs[k].Raw
		if k == len(selectedCombo)-1 {
			for i := k + 1; i < len(rawArgs); i++ {
				buf += " " + rawArgs[i].Raw // Add the rest of the parameters to the end
			}
		}

		switch sc.Arguments[comboArg].Type {
		case ArgumentTypeString:
			val = buf
		case ArgumentTypeNumber:
			val, err = ParseNumber(buf)
		case ArgumentTypeUser:
			if channel == nil || channel.IsPrivate {
				continue // can't provide users in direct messages
			}
			val, err = ParseUser(buf, m.Message, s)
		}

		if err != nil {
			return nil, errors.New("Failed parsing arguments: " + err.Error())
		}

		parsedArgs[comboArg] = &ParsedArgument{
			Raw:    buf,
			Parsed: val,
		}
	}

	return &ParsedCommand{
		Name: sc.Name,
		Cmd:  sc,
		Args: parsedArgs,
	}, nil
}

// Finds a proper argument combo from the provided args
func (sc *SimpleCommand) findCombo(rawArgs []*MatchedArg) ([]int, bool) {
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

func (sc *SimpleCommand) checkArgumentMatch(raw *MatchedArg, definition ArgumentType) bool {
	switch definition {
	case ArgumentTypeNumber:
		return raw.Type == ArgumentTypeNumber
	case ArgumentTypeUser:
		// Check if a user mention is required
		// Otherwise it can be of any type
		if sc.UserArgRequireMention {
			return raw.Type == ArgumentTypeUser
		} else {
			return true
		}
	case ArgumentTypeString:
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
	Raw  string
}

// Reads the command line and seperates it into a slice of strings
// These strings are later processed depending on the argument type they belong to
func ReadArgs(in string) []*MatchedArg {
	rawArgs := make([]string, 0)

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
					rawArgs = append(rawArgs, curBuf)
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
					rawArgs = append(rawArgs, curBuf)
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
		rawArgs = append(rawArgs, curBuf)
	}

	// Match up the arguments to possible datatypes
	// Used when finding the proper combo
	// Only distinguishes between numbers and strings atm
	// Which means it won't work properly if you have 2 combos
	// where the only differences are string and user
	// it will not work as expected
	out := make([]*MatchedArg, len(rawArgs))
	for i, raw := range rawArgs {
		// Check for number
		_, err := strconv.ParseFloat(raw, 64)
		if err == nil {
			out[i] = &MatchedArg{Type: ArgumentTypeNumber, Raw: raw}
			continue
		}
		if strings.Index(raw, "<@") == 0 {
			if raw[len(raw)-1] == '>' {
				// Mention, so user
				out[i] = &MatchedArg{Type: ArgumentTypeUser, Raw: raw}
				continue
			}
		}
		// Else it could be anything, no definitive answer
		out[i] = &MatchedArg{Type: ArgumentTypeString, Raw: raw}
	}

	return out
}

// Parses a discord user from buf and returns the error if any
func ParseUser(buf string, m *discordgo.Message, s *discordgo.Session) (user *discordgo.User, err error) {
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
		user, err = FindDiscordUser(field, m, s)
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
	return "???" // ????
}

type ArgumentDef struct {
	Name        string
	Description string
	Type        ArgumentType
}

func (a *ArgumentDef) String() string {
	return a.Name
}

// Holds parsed argument data
type ParsedArgument struct {
	Raw    string
	Parsed interface{}
}

// Helper to convert the data to an int
func (p *ParsedArgument) Int() int {
	val, _ := p.Parsed.(float64)
	return int(val)
}

// Helper to convert the data to a string
func (p *ParsedArgument) Str() string {
	val, _ := p.Parsed.(string)
	return val
}

// Helper to converty tht edata to a float64
func (p *ParsedArgument) Float() float64 {
	val, _ := p.Parsed.(float64)
	return val
}

// Helper to convert the data to a discorduser
func (p *ParsedArgument) DiscordUser() *discordgo.User {
	val, _ := p.Parsed.(*discordgo.User)
	return val
}

// Represents a parsedcommand
type ParsedCommand struct {
	Name    string
	Source  CommandSource
	Cmd     *SimpleCommand
	Args    []*ParsedArgument
	Channel *discordgo.Channel
	Guild   *discordgo.Guild
}
