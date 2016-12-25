package dstate

import (
	"github.com/bwmarrin/discordgo"
	"sync"
	"time"
)

type State struct {
	sync.RWMutex

	// All connected guilds
	Guilds map[string]*GuildState

	// Global channel mapping for convenience
	channels map[string]*ChannelState

	// Absolute max number of messages stored per channel
	MaxChannelMessages int

	// Max duration of messages stored, ignored if 0
	// (Messages gets checked when a new message in the channel comes in)
	MaxMessageAge time.Duration

	TrackChannels  bool
	TrackMembers   bool
	TrackRoles     bool
	TrackVoice     bool
	TrackPresences bool

	// Removes offline members from the state, requires trackpresences
	RemoveOfflineMembers bool

	// Set to remove deleted messages from state
	RemoveDeletedMessages bool
}

func NewState() *State {
	return &State{
		Guilds:   make(map[string]*GuildState),
		channels: make(map[string]*ChannelState),

		TrackChannels:         true,
		TrackMembers:          true,
		TrackRoles:            true,
		TrackVoice:            true,
		TrackPresences:        true,
		RemoveDeletedMessages: true,
	}
}

type MemberState struct {
	Guild *GuildState

	Member   *discordgo.Member
	Presence *discordgo.Presence
}

// ChannelState represents a channel's state
type ChannelState struct {
	Guild *GuildState

	Channel  *discordgo.Channel
	Messages []*MessageState
}

func (c *ChannelState) Message(mID string) *MessageState {
	for _, m := range c.Messages {
		if m.Message.ID == mID {
			return m
		}
	}

	return nil
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

func (s *State) Guild(lock bool, id string) *GuildState {
	if lock {
		s.RLock()
		defer s.RUnlock()
	}

	return s.Guilds[id]
}

func (s *State) Channel(lock bool, id string) *ChannelState {
	if lock {
		s.RLock()
		defer s.RUnlock()
	}

	return s.channels[id]
}

// Differantiate between
func (s *State) GuildCreate(lock bool, g *discordgo.Guild) {
	if lock {
		s.Lock()
		defer s.Unlock()
	}

	// Preserve messages in the state and
	// purge existing global channel maps if this guy was already in the state
	preservedMessages := make(map[string][]*MessageState)

	existing := s.Guild(false, g.ID)
	if existing != nil {
		// Synchronization is hard
		toRemove := make([]string, 0)
		s.Unlock()
		existing.RLock()
		for _, channel := range existing.Channels {
			preservedMessages[channel.Channel.ID] = channel.Messages
			toRemove = append(toRemove, channel.Channel.ID)
		}
		existing.RUnlock()
		s.Lock()

		for _, cID := range toRemove {
			delete(s.channels, cID)
		}
	}

	// No need to lock it since we just created it and theres no chance of anyone else accessing it
	guildState := NewGuildState(g, s)
	for _, channel := range guildState.Channels {
		if preserved, ok := preservedMessages[channel.Channel.ID]; ok {
			channel.Messages = preserved
		}

		s.channels[channel.Channel.ID] = channel
	}

	s.Guilds[g.ID] = guildState
}

func (s *State) GuildUpdate(g *discordgo.Guild) {

	guildState := s.Guild(true, g.ID)
	if guildState == nil {
		s.GuildCreate(true, g)
		return
	}

	guildState.GuildUpdate(true, g)
}

func (s *State) GuildRemove(id string) {
	s.Lock()
	defer s.Unlock()

	g := s.Guild(false, id)
	if g == nil {
		return
	}
	// Remove all references
	for c, cs := range s.channels {
		if cs.Guild == g {
			delete(s.channels, c)
		}
	}
	delete(s.Guilds, id)
}

func (s *State) HandleEvent(session *discordgo.Session, i interface{}) {
	switch evt := i.(type) {

	// Guild events
	case *discordgo.GuildCreate:
		s.GuildCreate(true, evt.Guild)
	case *discordgo.GuildUpdate:
		s.GuildUpdate(evt.Guild)
	case *discordgo.GuildDelete:
		s.GuildRemove(evt.ID)

	// Member events
	case *discordgo.GuildMemberAdd:
		if !s.TrackMembers {
			return
		}

		g := s.Guild(true, evt.GuildID)
		if g != nil {
			g.MemberAddUpdate(true, evt.Member)
		}
	case *discordgo.GuildMemberUpdate:
		if !s.TrackMembers {
			return
		}

		g := s.Guild(true, evt.GuildID)
		if g != nil {
			g.MemberAddUpdate(true, evt.Member)
		}
	case *discordgo.GuildMemberRemove:
		if !s.TrackMembers {
			return
		}

		g := s.Guild(true, evt.GuildID)
		if g != nil {
			g.MemberRemove(true, evt.User.ID)
		}

	// Channel events
	case *discordgo.ChannelCreate:
		if !s.TrackChannels {
			return
		}

		g := s.Guild(true, evt.GuildID)
		if g != nil {
			c := g.ChannelAddUpdate(true, evt.Channel)
			s.Lock()
			s.channels[evt.Channel.ID] = c
			s.Unlock()
		}
	case *discordgo.ChannelUpdate:
		if !s.TrackChannels {
			return
		}

		g := s.Guild(true, evt.GuildID)
		if g != nil {
			c := g.ChannelAddUpdate(true, evt.Channel)
			s.Lock()
			s.channels[evt.Channel.ID] = c
			s.Unlock()
		}
	case *discordgo.ChannelDelete:
		if !s.TrackChannels {
			return
		}

		g := s.Guild(true, evt.GuildID)
		if g != nil {
			g.ChannelAddUpdate(true, evt.Channel)
			s.Lock()
			delete(s.channels, evt.Channel.ID)
			s.Unlock()
		}

	// Role events
	case *discordgo.GuildRoleCreate:
		if !s.TrackRoles {
			return
		}

		g := s.Guild(true, evt.GuildID)
		if g != nil {
			g.RoleAddUpdate(true, evt.Role)
		}
	case *discordgo.GuildRoleUpdate:
		if !s.TrackRoles {
			return
		}

		g := s.Guild(true, evt.GuildID)
		if g != nil {
			g.RoleAddUpdate(true, evt.Role)
		}
	case *discordgo.GuildRoleDelete:
		if !s.TrackRoles {
			return
		}

		g := s.Guild(true, evt.GuildID)
		if g != nil {
			g.RoleRemove(true, evt.RoleID)
		}

	// Message events
	case *discordgo.MessageCreate:
		channel := s.Channel(true, evt.ChannelID)
		if channel != nil {
			channel.Guild.MessageAddUpdate(true, evt.Message)
		}
	case *discordgo.MessageUpdate:
		channel := s.Channel(true, evt.ChannelID)
		if channel != nil {
			channel.Guild.MessageAddUpdate(true, evt.Message)
		}
	case *discordgo.MessageDelete:
		channel := s.Channel(true, evt.ChannelID)
		if channel != nil {
			channel.Guild.MessageRemove(true, evt.ChannelID, evt.Message.ID)
		}

	// Other
	case *discordgo.PresenceUpdate:
		if !s.TrackPresences {
			return
		}

		g := s.Guild(true, evt.GuildID)
		if g != nil {
			g.PresenceAddUpdate(true, &evt.Presence)
		}
	case *discordgo.VoiceStateUpdate:
		if !s.TrackVoice {
			return
		}
		g := s.Guild(true, evt.GuildID)
		if g != nil {
			g.VoiceStateUpdate(true, evt)
		}
	}

}
