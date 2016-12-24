package main

import (
	"flag"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"log"
)

var (
	flagToken string
)

func init() {
	flag.StringVar(&flagToken, "t", "", "Token to use")

	flag.Parse()
}

func main() {
	session, err := discordgo.New(flagToken)
	if err != nil {
		panic(err)
	}

	// Create a new command system, this function will add a handler
	// and set the prfix to "!"
	system := commandsystem.NewSystem(session, "!")

	// Add general commands
	Addcommands(system)

	system.DefaultDMHandler = &commandsystem.Command{
		Arguments: []*commandsystem.ArgDef{
			&commandsystem.ArgDef{Name: "Extra stuff", Type: commandsystem.ArgumentString, Default: "Nothing"},
		},
		Run: func(data *commandsystem.ExecData) (interface{}, error) {
			return "You called this from from a direct message and no command matched! Extra stuff: " + data.Args[0].Str(), nil
		},
	}
	system.DefaultMentionHandler = &commandsystem.Command{
		Run: func(data *commandsystem.ExecData) (interface{}, error) {
			return "You mentioned the bot and no command was found!", nil
		},
	}
	system.DefaultHandler = &commandsystem.Command{
		Run: func(data *commandsystem.ExecData) (interface{}, error) {
			return "You used the prefix but no command was found!", nil
		},
	}

	session.AddHandler(HandleReady)
	session.AddHandler(HandleServerJoin)

	err = session.Open()
	if err != nil {
		log.Fatal("Failed opening websocket connection")
	}

	log.Println("Started example bot :D stop with ctrl-c")
	select {}
}

func HandleReady(s *discordgo.Session, r *discordgo.Ready) {
	log.Println("Ready received! Connected to", len(r.Guilds), "Guilds")
}

func HandleServerJoin(s *discordgo.Session, g *discordgo.GuildCreate) {
	log.Println("Joined guild", g.Name, " Connected to", len(s.State.Guilds), "Guilds")
}

func Addcommands(system *commandsystem.System) {
	echoCmd := &commandsystem.Command{
		Name:        "Echo",
		Description: "Christmas is coming soon",
		Arguments: []*commandsystem.ArgDef{
			&commandsystem.ArgDef{Name: "what", Type: commandsystem.ArgumentString},
		},
		RequiredArgs: 1,
		Run: func(data *commandsystem.ExecData) (interface{}, error) {
			return data.Args[0].Str(), nil
		},
	}

	funcCommands := []commandsystem.CommandHandler{
		&commandsystem.Command{
			Name:            "Hey",
			Description:     "Nice greeting",
			LongDescription: "This long description will only be shown when you do '!help hey'\nIt's nice to use for indepth examples and whatnot",
			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				return "Hello there, how was your day?", nil
			},
		},
		&commandsystem.Command{
			Name:        "How",
			Description: "What is this computer code thing what am doign halp",
			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				return "How is this working?", nil
			},
		},
		&commandsystem.CommandContainer{
			Name:        "Nested",
			Description: "This is a nested command container",
			Children: []commandsystem.CommandHandler{
				echoCmd,
			},
		},
	}

	cmdInvite := &commandsystem.Command{
		Name:        "Invite",
		Description: "Responds with a bot invite",
		RunInDm:     true,
		Run: func(data *commandsystem.ExecData) (interface{}, error) {
			return "You smell bad https://discordapp.com/oauth2/authorize?client_id=&scope=bot&permissions=101376", nil
		},
	}

	helpCmd := &commandsystem.Command{
		Name:        "Help",
		Description: "Shows help abut all or one specific command",
		Arguments: []*commandsystem.ArgDef{
			// Set default to be not nil when no command is specified, so that the parsed command is always not nil
			&commandsystem.ArgDef{Name: "command", Type: commandsystem.ArgumentString, Default: ""},
		},
		Run: func(data *commandsystem.ExecData) (interface{}, error) {
			target := data.Args[0].Str()

			// Second argument is depth, how many nested command contains the help generation will go into
			help := system.GenerateHelp(target, 100)

			return help, nil
		},
	}

	commands := []commandsystem.CommandHandler{
		// The commands in this container can be ran using
		// !fun (command name)
		// funCommands also holds another command container with echo inside
		// to call that echo command you would have to do
		// !fun nested echo this will be echoed back
		&commandsystem.CommandContainer{
			Name:        "Fun",
			Description: "Fun container",
			Children:    funcCommands,
		},
		echoCmd,
		cmdInvite,
		helpCmd,
	}
	system.RegisterCommands(commands...)
}
