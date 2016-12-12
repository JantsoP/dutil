package commandsystem

import (
	"context"
	"errors"
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil"
	"time"
)

type CommandResponse interface {
	// Channel, session, command etc can all be found in this context
	Send(ctx context.Context) ([]*discordgo.Message, error)
}

func SendResponseInterface(ctx context.Context, reply interface{}) ([]*discordgo.Message, error) {
	s := CtxSession(ctx)
	c := CtxChannel(ctx)
	switch t := reply.(type) {
	case CommandResponse:
		return t.Send(ctx)
	case string:
		return dutil.SplitSendMessage(s, c.ID, t)
	case error:
		return dutil.SplitSendMessage(s, c.ID, t.Error())
	case *discordgo.MessageEmbed:
		m, err := s.ChannelMessageSendEmbed(c.ID, t)
		return []*discordgo.Message{m}, err
	}

	cmd := ctx.Value(KeyCommand)
	cmdName := "?"
	if cmd != nil {
		cmdName = cmd.(*Command).Name
	}

	return nil, errors.New("Unknown reply type in '" + cmdName + "'")
}

// Temporary response deletes the inner response after Duration
type TemporaryResponse struct {
	Response interface{}
	Duration time.Duration
}

func NewTemporaryResponse(d time.Duration, inner interface{}) *TemporaryResponse {
	return &TemporaryResponse{
		Duration: d,
		Response: inner,
	}
}

func (t *TemporaryResponse) Send(ctx context.Context) ([]*discordgo.Message, error) {
	session := CtxSession(ctx)
	channel := CtxChannel(ctx)

	msgs, err := SendResponseInterface(ctx, t.Response)
	if err != nil {
		return nil, err
	}
	time.AfterFunc(t.Duration, func() {
		// do a bulk if 2 or more
		if len(msgs) > 1 {
			ids := make([]string, len(msgs))
			for i, m := range msgs {
				ids[i] = m.ID
			}
			session.ChannelMessagesBulkDelete(channel.ID, ids)
		} else {
			session.ChannelMessageDelete(channel.ID, msgs[0].ID)
		}
	})
	return msgs, nil
}

// The FallbackEmbed reponse type will turn the embed into a normal mesasge if there is not enough permissions
// This requires state member tracking enabled
type FallbackEmebd struct {
	*discordgo.MessageEmbed
}

func (fe *FallbackEmebd) Send(ctx context.Context) ([]*discordgo.Message, error) {
	session := CtxSession(ctx)
	channel := CtxChannel(ctx)

	channelPerms, err := session.State.UserChannelPermissions(session.State.User.ID, channel.ID)
	if err != nil {
		return nil, err
	}

	if channelPerms&discordgo.PermissionEmbedLinks != 0 {
		m, err := session.ChannelMessageSendEmbed(channel.ID, fe.MessageEmbed)
		if err != nil {
			return nil, err
		}

		return []*discordgo.Message{m}, nil
	}

	content := StringEmbed(fe.MessageEmbed) + "\n**I have no 'embed links' permissions here, this is a fallback. it looks prettier if i have that perm :)**"
	return dutil.SplitSendMessage(session, channel.ID, content)
}

func StringEmbed(embed *discordgo.MessageEmbed) string {
	body := ""

	if embed.Author != nil {
		body += embed.Author.Name + "\n"
		body += embed.Author.URL + "\n"
	}

	if embed.Title != "" {
		body += "**" + embed.Title + "**\n"
	}

	if embed.Description != "" {
		body += embed.Description + "\n"
	}
	if body != "" {
		body += "\n"
	}

	for _, v := range embed.Fields {
		body += fmt.Sprintf("**%s**\n%s\n\n", v.Name, v.Value)
	}
	return body
}
