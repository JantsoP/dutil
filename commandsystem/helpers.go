package commandsystem

import (
	"context"
	"github.com/jonas747/discordgo"
)

func CtxSession(ctx context.Context) *discordgo.Session {
	return ctx.Value(KeySession).(*discordgo.Session)
}

func CtxMessage(ctx context.Context) *discordgo.MessageCreate {
	return ctx.Value(KeyMessage).(*discordgo.MessageCreate)
}

func CtxCommand(ctx context.Context) *Command {
	return ctx.Value(KeyCommand).(*Command)
}

func CtxSource(ctx context.Context) CommandSource {
	return ctx.Value(KeySource).(CommandSource)
}

func CtxArgs(ctx context.Context) []*ParsedArgument {
	return ctx.Value(KeyArgs).([]*ParsedArgument)
}

func CtxGuild(ctx context.Context) *discordgo.Guild {
	return ctx.Value(KeyGuild).(*discordgo.Guild)
}
func CtxChannel(ctx context.Context) *discordgo.Channel {
	return ctx.Value(KeyChannel).(*discordgo.Channel)
}
