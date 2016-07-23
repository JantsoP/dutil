package main

import (
	"flag"
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"log"
)

var (
	flagToken string
	dgo       *discordgo.Session
)

func init() {
	flag.StringVar(&flagToken, "t", "", "Token to use")

	if !flag.Parsed() {
		flag.Parse()
	}
}

func main() {
	session, err := discordgo.New(flagToken)
	if err != nil {
		panic(err)
	}
	dgo = session

	system := commandsystem.NewSystem(session, ":)")

	Addcommands(system)

	dgo.AddHandler(HandleMessageCreate)
	dgo.AddHandler(HandleReady)
	dgo.AddHandler(HandleServerJoin)

	err = dgo.Open()
	if err != nil {
		panic(err)
	}
	log.Println("Started example bot :D")
	select {}
}

func HandleReady(s *discordgo.Session, r *discordgo.Ready) {
	log.Println("Ready received! Connected to", len(s.State.Guilds), "Guilds")
}

func HandleServerJoin(s *discordgo.Session, g *discordgo.GuildCreate) {
	log.Println("Joined guild", g.Name, " Connected to", len(s.State.Guilds), "Guilds")
}

func HandleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

}

func Addcommands(system *commandsystem.System) {
	echoCmd := &commandsystem.SimpleCommand{
		Name: "echo",
		Arguments: []*commandsystem.ArgumentDef{
			&commandsystem.ArgumentDef{Name: "what", Type: commandsystem.ArgumentTypeString},
		},
		RequiredArgs: 1,
		RunFunc: func(cmd *commandsystem.ParsedCommand, source commandsystem.CommandSource, m *discordgo.MessageCreate) error {
			dgo.ChannelMessageSend(m.ChannelID, cmd.Args[0].Str())
			return nil
		},
	}

	funcCommands := []commandsystem.CommandHandler{
		&commandsystem.SimpleCommand{
			Name:        "Hey",
			Description: "Nice greeting",
			RunFunc: func(cmd *commandsystem.ParsedCommand, source commandsystem.CommandSource, m *discordgo.MessageCreate) error {
				dgo.ChannelMessageSend(m.ChannelID, "Wassup")
				return nil
			},
		},
		&commandsystem.SimpleCommand{
			Name:        "How",
			Description: "What is this computer code thing what am doign halp",
			RunFunc: func(cmd *commandsystem.ParsedCommand, source commandsystem.CommandSource, m *discordgo.MessageCreate) error {
				dgo.ChannelMessageSend(m.ChannelID, "Wassup")
				return nil
			},
		},
		&commandsystem.CommandContainer{
			Name:        "nested",
			Description: "This is a nested command container",
			Children: []commandsystem.CommandHandler{
				echoCmd,
			},
		},
	}

	cmdInvite := &commandsystem.SimpleCommand{
		Name:    "invite",
		RunInDm: true,
		RunFunc: func(parsed *commandsystem.ParsedCommand, source commandsystem.CommandSource, m *discordgo.MessageCreate) error {
			dgo.ChannelMessageSend(m.ChannelID, "You smell bad https://discordapp.com/oauth2/authorize?client_id=&scope=bot&permissions=101376")
			return nil
		},
	}

	helpCmd := &commandsystem.SimpleCommand{
		Name:        "Help",
		Description: "Shows help abut all or one specific command",
		Arguments: []*commandsystem.ArgumentDef{
			&commandsystem.ArgumentDef{Name: "command", Type: commandsystem.ArgumentTypeString},
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, source commandsystem.CommandSource, m *discordgo.MessageCreate) error {
			target := ""
			if parsed.Args[0] != nil {
				target = parsed.Args[0].Str()
			}
			help := system.GenerateHelp(target, 0)
			dgo.ChannelMessageSend(m.ChannelID, help)
			return nil
		},
	}

	commands := []commandsystem.CommandHandler{
		&commandsystem.CommandContainer{
			Name:        "fun",
			Description: "Fun container",
			Children:    funcCommands,
		},
		echoCmd,
		cmdInvite,
		helpCmd,
	}
	system.RegisterCommands(commands...)
}
