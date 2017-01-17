package commandsystem

import (
	"context"
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dutil/dstate"
	"strings"
)

var (
	// Returned if the parameters passed to the command didnt match the command definition
	ErrInvalidParameters   = errors.New("Invalid parameters passed to command, see help for usage")
	ErrDiscordUserNotFound = errors.New("Discord user not found")
)

type CommandHandlerFunc func(raw string, m *discordgo.MessageCreate, s *discordgo.Session)

type TriggerData struct {
	Session *discordgo.Session

	// Nil if dstate is not being used
	DState *dstate.State

	// Message that triggered the command or nil if none
	Message *discordgo.Message
	Source  Source
}

// Represents a command handler to handle commands
type CommandHandler interface {
	// Called to check if the command matched "raw"
	CheckMatch(raw string, triggerData *TriggerData) bool

	// Handle the command itself, returns the response messages and an error if something went wrong
	HandleCommand(raw string, triggerData *TriggerData, ctx context.Context) ([]*discordgo.Message, error)

	// Generates help output, maxDepth is how far into container help will go
	GenerateHelp(target string, maxDepth, currentDepth int) string
}

// A general purpose CommandHandler implementation
// With support for aliases, automatically parsed arguments
// with different combos, generated help and optionally ran in dm
//
// Argument combos
// Argument combos are a way to make the order of arguments dynamic, you should be carefull with these
// and not use them for any arguement types that can't be distinguished from eachtother
//
// Say you have a command that takes 2 arguments, a discord user and a number
// since these values can easily be distinguished (UserArgRequireMention is set)
// you can set it up like this:
// arg 0: user arg
// arg 1: number arg
// combos: [][]int{[]int{0,1}, []int{1,0}}
// (the number in combos referrs to the arguement index)
// and now arg 0 will always be a user and and arg 1 will always be a number arg
// no matter the order
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
type Command struct {
	Name            string   // Name of command, what its called from
	Aliases         []string // Aliases which it can also be called from
	Description     string   // Description shown in non targetted help
	LongDescription string   // Longer description when this command was targetted

	HideFromHelp            bool // Hide it from help
	UserArgRequireMention   bool // Set to require user mention in user mentions, otherwise it will attempt to search
	IgnoreUserNotFoundError bool // Instead of throwing a User not found error, it will ignore it if it's not a required argument

	RunInDm        bool // Run in dms, users can't be provided as arguments then
	IgnoreMentions bool // Will not be triggered by mentions

	Arguments      []*ArgDef // Slice of argument definitions, ctx.Args will always be the same size as this slice (although the data may be nil)
	RequiredArgs   int       // Number of reuquired arguments, ignored if combos is specified
	ArgumentCombos [][]int   // Slice of argument pairs, will override RequiredArgs if specified

	// Run is ran the the command has sucessfully been parsed
	// It returns a reply and an error
	// the reply can have a type of string, *MessageEmbed or error
	Run RunFunc
}

type RunFunc func(data *ExecData) (interface{}, error)

func (sc *Command) GenerateHelp(target string, maxDepth, currentDepth int) string {
	if target != "" {
		if !sc.CheckMatch(target, &TriggerData{Source: SourceHelp}) {
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
	if target != "" && sc.LongDescription != "" {
		out += "\n" + sc.LongDescription
	}
	return out
}

func (sc *Command) CheckMatch(raw string, triggerData *TriggerData) bool {
	// Check if this is a mention and ignore if so
	if triggerData.Source == SourceMention && sc.IgnoreMentions {
		return false
	}

	// Same as above with dm's
	if triggerData.Source == SourceDM && !sc.RunInDm {
		return false
	}

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

	return true
}

func (sc *Command) HandleCommand(raw string, triggerData *TriggerData, ctx context.Context) (msgs []*discordgo.Message, err error) {
	parsedData, err := sc.ParseCommand(raw, triggerData)
	if err != nil {
		triggerData.Session.ChannelMessageSend(triggerData.Message.ChannelID, "Failed parsing command: "+err.Error())
		return nil, err
	}

	parsedData.ctx = ctx
	parsedData.Source = triggerData.Source

	if sc.Run == nil {
		return nil, nil
	}

	reply, err := sc.Run(parsedData)
	if reply != nil {
		var err2 error
		msgs, err2 = SendResponseInterface(parsedData, reply)
		if err2 != nil {
			return msgs, err2
		}
	}

	// Command error
	return
}

type ArgumentType int

const (
	ArgumentString ArgumentType = iota
	ArgumentNumber
	ArgumentUser
)

func (a ArgumentType) String() string {
	switch a {
	case ArgumentString:
		return "String"
	case ArgumentNumber:
		return "Number"
	case ArgumentUser:
		return "@User"
	}
	return "???" // ????
}

type ArgDef struct {
	Name        string
	Description string
	Type        ArgumentType
	Default     interface{}
}

func (a *ArgDef) String() string {
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

type ExecData struct {
	Command CommandHandler
	Source  Source
	Args    []*ParsedArgument

	Session *discordgo.Session
	Message *discordgo.Message
	Guild   *dstate.GuildState
	Channel *dstate.ChannelState
	State   *dstate.State

	ctx context.Context
}

// Context returns an always non-nil context
func (e *ExecData) Context() context.Context {
	if e.ctx == nil {
		return context.Background()
	}

	return e.ctx
}

// WithContext Returns a copy of execdata with the context similar to net/http.Request.WithContext
func (e *ExecData) WithContext(ctx context.Context) *ExecData {
	ne := new(ExecData)
	*ne = *e
	ne.ctx = ctx

	return ne
}
