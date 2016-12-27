package dstate

import (
	"github.com/bwmarrin/discordgo"
	"time"
)

// ChannelState represents a channel's state
type ChannelState struct {
	Owner RWLocker
	Guild *GuildState

	Channel  *discordgo.Channel
	Messages []*MessageState
}

func (c *ChannelState) Update(newChannel *discordgo.Channel) {
	if newChannel.PermissionOverwrites == nil {
		newChannel.PermissionOverwrites = c.Channel.PermissionOverwrites
	}
	if newChannel.IsPrivate && newChannel.Recipient == nil {
		newChannel.Recipient = c.Channel.Recipient
	}
	*c.Channel = *newChannel
}

// Copy returns a copy of the channel
// if deep is true, permissionoverwrites will be copied
func (c *ChannelState) Copy(lock bool, deep bool) *discordgo.Channel {
	if lock {
		c.Owner.RLock()
		defer c.Owner.RUnlock()
	}

	cCopy := new(discordgo.Channel)
	*cCopy = *c.Channel

	cCopy.Messages = nil
	cCopy.PermissionOverwrites = nil

	if deep {
		for _, pow := range c.Channel.PermissionOverwrites {
			powCopy := new(discordgo.PermissionOverwrite)
			*powCopy = *pow
			cCopy.PermissionOverwrites = append(cCopy.PermissionOverwrites, pow)
		}
	}

	return cCopy
}

func (c *ChannelState) Message(mID string) *MessageState {
	for _, m := range c.Messages {
		if m.Message.ID == mID {
			return m
		}
	}

	return nil
}

func (c *ChannelState) MessageAddUpdate(msg *discordgo.Message, maxMessages int, maxMessageAge time.Duration) {

	existing := c.Message(msg.ID)
	if existing != nil {
		// Patch the existing message
		if msg.Content != "" {
			existing.Message.Content = msg.Content
		}
		if msg.EditedTimestamp != "" {
			existing.Message.EditedTimestamp = msg.EditedTimestamp
		}
		if msg.Mentions != nil {
			existing.Message.Mentions = msg.Mentions
		}
		if msg.Embeds != nil {
			existing.Message.Embeds = msg.Embeds
		}
		if msg.Attachments != nil {
			existing.Message.Attachments = msg.Attachments
		}
		if msg.Timestamp != "" {
			existing.Message.Timestamp = msg.Timestamp
		}
		if msg.Author != nil {
			existing.Message.Author = msg.Author
		}
		existing.ParseTimes()
	} else {
		// Add the new one
		ms := &MessageState{
			Message: msg,
		}
		ms.ParseTimes()
		c.Messages = append(c.Messages, ms)
		if len(c.Messages) > maxMessages {
			c.Messages = c.Messages[len(c.Messages)-maxMessages:]
		}
	}

	// Check age
	if maxMessageAge == 0 {
		return
	}

	now := time.Now()
	for i := len(c.Messages) - 1; i >= 0; i-- {
		m := c.Messages[i]

		ts := m.ParsedCreated
		if ts.IsZero() {
			continue
		}

		if now.Sub(ts) > maxMessageAge {
			// All messages before this is old aswell
			// TODO: remove by edited timestamp if set
			c.Messages = c.Messages[i:]
			break
		}
	}
}

// MessageRemove removes a message from the channelstate
// If mark is true the the message will just be marked as deleted and not removed from the state
func (c *ChannelState) MessageRemove(messageID string, mark bool) {

	for i, ms := range c.Messages {
		if ms.Message.ID == messageID {
			if !mark {
				c.Messages = append(c.Messages[:i], c.Messages[i+1:]...)
			} else {
				ms.Deleted = true
			}
			return
		}
	}
}
