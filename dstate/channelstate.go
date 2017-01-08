package dstate

import (
	"github.com/jonas747/discordgo"
	"time"
)

// ChannelState represents a channel's state
type ChannelState struct {
	Owner RWLocker
	Guild *GuildState

	// These fields are never mutated and can be accessed without locking
	id        string
	kind      string
	recipient *discordgo.User
	isPrivate bool

	// Accessing the channel without locking the owner yields undefined behaviour
	Channel  *discordgo.Channel
	Messages []*MessageState
}

func NewChannelState(guild *GuildState, owner RWLocker, channel *discordgo.Channel) *ChannelState {

	// Create a copy of the channel
	cCopy := copyChannel(channel, true)

	cs := &ChannelState{
		Owner:   owner,
		Guild:   guild,
		Channel: cCopy,

		id:        channel.ID,
		kind:      channel.Type,
		isPrivate: channel.IsPrivate,
	}

	if channel.IsPrivate && channel.Recipient != nil {
		// Make a copy of the recipient
		rec := new(discordgo.User)
		*rec = *channel.Recipient
		cs.recipient = rec
	}

	return cs
}

// Set of accessors below to access the immutable fields and make sure you can't modify them

// ID returns the channels id
// This does no locking as ID is immutable
func (cs *ChannelState) ID() string {
	return cs.id
}

// Type returns the channels type
// This does no locking as Type is immutable
func (cs *ChannelState) Type() string {
	return cs.kind
}

// Recipient returns the channels recipient, if you modify this you get undefined behaviour
// This does no locking as Recipient is immutable
func (cs *ChannelState) Recipient() *discordgo.User {
	return cs.recipient
}

// IsPrivate returns true if the channel is private
// This does no locking as IsPrivate is immutable
func (cs *ChannelState) IsPrivate() bool {
	return cs.isPrivate
}

// Copy returns a copy of the channel
// if deep is true, permissionoverwrites will be copied
func (c *ChannelState) Copy(lock bool, deep bool) *discordgo.Channel {
	if lock {
		c.Owner.RLock()
		defer c.Owner.RUnlock()
	}

	return copyChannel(c.Channel, deep)
}

func copyChannel(in *discordgo.Channel, deep bool) *discordgo.Channel {
	cCopy := new(discordgo.Channel)
	*cCopy = *in

	cCopy.Messages = nil
	cCopy.PermissionOverwrites = nil

	if deep {
		for _, pow := range in.PermissionOverwrites {
			powCopy := new(discordgo.PermissionOverwrite)
			*powCopy = *pow
			cCopy.PermissionOverwrites = append(cCopy.PermissionOverwrites, pow)
		}
	}

	return cCopy
}

// Update updates a channel
// Undefined behaviour if owner (guild or state) is not locked
func (c *ChannelState) Update(lock bool, newChannel *discordgo.Channel) {
	if lock {
		c.Owner.Lock()
		defer c.Owner.Unlock()
	}

	if newChannel.PermissionOverwrites == nil {
		newChannel.PermissionOverwrites = c.Channel.PermissionOverwrites
	}
	if newChannel.IsPrivate && newChannel.Recipient == nil {
		newChannel.Recipient = c.Channel.Recipient
	}

	*c.Channel = *newChannel
}

// Message returns a message by id or nil if none found
// The only field safe to query on a message without locking the owner (guild or state) is ID
func (c *ChannelState) Message(lock bool, mID string) *MessageState {
	if lock {
		c.Owner.RLock()
		defer c.Owner.RUnlock()
	}

	for _, m := range c.Messages {
		if m.Message.ID == mID {
			return m
		}
	}

	return nil
}

// MessageAddUpdate adds or updates an existing message
func (c *ChannelState) MessageAddUpdate(lock bool, msg *discordgo.Message, maxMessages int, maxMessageAge time.Duration) {
	if lock {
		c.Owner.Lock()
		defer c.Owner.Unlock()
	}

	defer c.UpdateMessages(false, maxMessages, maxMessageAge)

	existing := c.Message(false, msg.ID)
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
		// make a copy
		// No need to copy author aswell as that isnt mutated
		msgCopy := new(discordgo.Message)
		*msgCopy = *msg

		// Add the new one
		ms := &MessageState{
			Message: msgCopy,
		}

		ms.ParseTimes()
		c.Messages = append(c.Messages, ms)
	}
}

// UpdateMessages checks the messages to make sure they fit max message age and max messages
func (c *ChannelState) UpdateMessages(lock bool, maxMsgs int, maxAge time.Duration) {
	if lock {
		c.Owner.Lock()
		defer c.Owner.Unlock()
	}

	if len(c.Messages) > maxMsgs && maxMsgs != -1 {
		c.Messages = c.Messages[len(c.Messages)-maxMsgs:]
	}

	// Check age
	if maxAge == 0 {
		return
	}

	now := time.Now()
	for i := len(c.Messages) - 1; i >= 0; i-- {
		m := c.Messages[i]

		ts := m.ParsedCreated
		if ts.IsZero() {
			continue
		}

		if now.Sub(ts) > maxAge {
			// All messages before this is old aswell
			// TODO: remove by edited timestamp if set
			c.Messages = c.Messages[i:]
			break
		}
	}
}

// MessageRemove removes a message from the channelstate
// If mark is true the the message will just be marked as deleted and not removed from the state
func (c *ChannelState) MessageRemove(lock bool, messageID string, mark bool) {
	if lock {
		c.Owner.Lock()
		defer c.Owner.Unlock()
	}

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

// MessageState represents the state of a message
type MessageState struct {
	Message *discordgo.Message

	// Set it the message was deleted
	Deleted bool

	// The parsed times below are cached because parsing all messages
	// timestamps in state everytime a new one came in would be stupid
	ParsedCreated time.Time
	ParsedEdited  time.Time
}

// ParseTimes parses the created and edited timestamps
func (m *MessageState) ParseTimes() {
	// The discord api is stopid, and edits can come before creates
	// Can also be handled before even if received in order cause of goroutines and scheduling
	if m.Message.Timestamp != "" {
		parsedC, _ := m.Message.Timestamp.Parse()
		m.ParsedCreated = parsedC
	}

	if m.Message.EditedTimestamp != "" {
		parsedE, _ := m.Message.EditedTimestamp.Parse()
		m.ParsedEdited = parsedE
	}
}

// Copy returns a copy of the message, that can be used without further locking the owner
func (m *MessageState) Copy(deep bool) *discordgo.Message {
	mCopy := new(discordgo.Message)
	*mCopy = *m.Message

	mCopy.Author = nil
	mCopy.Attachments = nil
	mCopy.Embeds = nil
	mCopy.MentionRoles = nil
	mCopy.Mentions = nil
	mCopy.Reactions = nil

	if !deep {
		return mCopy
	}

	if m.Message.Author != nil {
		mCopy.Author = new(discordgo.User)
		*mCopy.Author = *m.Message.Author
	}

	mCopy.Attachments = append(mCopy.Attachments, m.Message.Attachments...)
	mCopy.Embeds = append(mCopy.Embeds, m.Message.Embeds...)
	mCopy.Reactions = append(mCopy.Reactions, m.Message.Reactions...)

	mCopy.MentionRoles = append(mCopy.MentionRoles, m.Message.MentionRoles...)
	mCopy.Mentions = append(mCopy.Mentions, m.Message.Mentions...)

	return mCopy
}
