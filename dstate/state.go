package dstate

import (
	"github.com/bwmarrin/discordgo"
	"log"
	"reflect"
	"sync"
	"time"
)

type State struct {
	sync.RWMutex

	r *discordgo.Ready

	// All connected guilds
	Guilds map[string]*GuildState

	// Global channel mapping for convenience
	channels        map[string]*ChannelState
	PrivateChannels map[string]*ChannelState

	// Absolute max number of messages stored per channel
	MaxChannelMessages int

	// Max duration of messages stored, ignored if 0
	// (Messages gets checked when a new message in the channel comes in)
	MaxMessageAge time.Duration

	TrackChannels       bool
	TrackMembers        bool
	TrackRoles          bool
	TrackVoice          bool
	TrackPresences      bool
	ThrowAwayDMMessages bool // Don't track dm messages if set

	// Removes offline members from the state, requires trackpresences
	RemoveOfflineMembers bool

	// Set to remove deleted messages from state
	RemoveDeletedMessages bool

	// Enabled debug logging
	Debug bool
}

func NewState() *State {
	return &State{
		Guilds:          make(map[string]*GuildState),
		channels:        make(map[string]*ChannelState),
		PrivateChannels: make(map[string]*ChannelState),

		TrackChannels:         true,
		TrackMembers:          true,
		TrackRoles:            true,
		TrackVoice:            true,
		TrackPresences:        true,
		RemoveDeletedMessages: true,
		ThrowAwayDMMessages:   true,
	}
}

type MemberState struct {
	Guild *GuildState

	id string

	Member   *discordgo.Member
	Presence *discordgo.Presence
}

// ID returns the id of the member, this is safe to use without any locking as id is immutable
func (m *MemberState) ID() string {
	return m.id
}

// Guild returns a given guilds GuildState
func (s *State) Guild(lock bool, id string) *GuildState {
	if lock {
		s.RLock()
		defer s.RUnlock()
	}

	return s.Guilds[id]
}

// LightGuildcopy returns a light copy of a guild (without any slices included)
func (s *State) LightGuildCopy(lock bool, id string) *discordgo.Guild {
	if lock {
		s.RLock()
	}

	guild := s.Guild(false, id)
	if guild == nil {
		if lock {
			s.RUnlock()
		}
		return nil
	}

	if lock {
		s.RUnlock()
	}

	return guild.LightCopy(true)
}

// Channel returns a channelstate from id
func (s *State) Channel(lock bool, id string) *ChannelState {
	if lock {
		s.RLock()
		defer s.RUnlock()
	}

	return s.channels[id]
}

// ChannelCopy returns a copy of a channel,
// lock dictates wether state should be RLocked or not, channel will be locked regardless
func (s *State) ChannelCopy(lock bool, id string, deep bool) *discordgo.Channel {

	cState := s.Channel(lock, id)
	if cState == nil {
		return nil
	}

	return cState.Copy(true, deep)
}

// Differantiate between create and update
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
			preservedMessages[channel.ID()] = channel.Messages
			toRemove = append(toRemove, channel.ID())
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
		if preserved, ok := preservedMessages[channel.ID()]; ok {
			channel.Messages = preserved
		}

		s.channels[channel.ID()] = channel
	}

	s.Guilds[g.ID] = guildState
}

func (s *State) GuildUpdate(lockMain bool, g *discordgo.Guild) {
	guildState := s.Guild(lockMain, g.ID)
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

func (s *State) HandleReady(r *discordgo.Ready) {
	s.Lock()
	defer s.Unlock()

	s.r = r

	for _, channel := range r.PrivateChannels {
		cs := NewChannelState(nil, &sync.RWMutex{}, channel)
		s.channels[channel.ID] = cs
		s.PrivateChannels[channel.ID] = cs
	}

	for _, v := range r.Guilds {
		if s.Guild(false, v.ID) == nil {
			s.GuildCreate(false, v)
		} else {
			s.GuildUpdate(false, v)
		}
	}
}

// User Returns a copy of the user from the ready event
func (s *State) User(lock bool) *discordgo.User {
	if lock {
		s.RLock()
		defer s.RUnlock()
	}

	if s.r == nil || s.r.User == nil {
		return nil
	}

	uCopy := new(discordgo.User)
	*uCopy = *s.r.User

	return uCopy
}

func (s *State) ChannelAddUpdate(newChannel *discordgo.Channel) {
	if !s.TrackChannels {
		return
	}

	c := s.Channel(true, newChannel.ID)
	if c != nil {
		c.Update(true, newChannel)
		return
	}

	if !newChannel.IsPrivate {
		g := s.Guild(true, newChannel.GuildID)
		if g != nil {
			c = g.ChannelAddUpdate(true, newChannel)
		}
	} else {
		// Belongs to no guild, so we can create a new rwmutex
		c = NewChannelState(nil, &sync.RWMutex{}, newChannel)
	}

	s.Lock()
	s.channels[newChannel.ID] = c
	if newChannel.IsPrivate {
		s.PrivateChannels[newChannel.ID] = c
	}
	s.Unlock()
}

func (s *State) ChannelRemove(evt *discordgo.Channel) {
	if !s.TrackChannels {
		return
	}

	if evt.IsPrivate {
		s.Lock()
		defer s.Unlock()

		delete(s.channels, evt.ID)
		delete(s.PrivateChannels, evt.ID)
		return
	}

	g := s.Guild(true, evt.GuildID)
	if g != nil {
		g.ChannelRemove(true, evt.ID)

		s.Lock()
		delete(s.channels, evt.ID)
		s.Unlock()
	}
}

func (s *State) HandleEvent(session *discordgo.Session, i interface{}) {

	handled := false
	if s.Debug {
		t := reflect.Indirect(reflect.ValueOf(i)).Type()
		defer func() {
			if !handled {
				log.Printf("Did not handle, or had issues with %s; %#v", t.Name(), i)
			}
		}()
		log.Println("Inc event ", t.Name())
	}

	switch evt := i.(type) {

	// Guild events
	case *discordgo.GuildCreate:
		s.GuildCreate(true, evt.Guild)
	case *discordgo.GuildUpdate:
		s.GuildUpdate(true, evt.Guild)
	case *discordgo.GuildDelete:
		s.GuildRemove(evt.ID)

	// Member events
	case *discordgo.GuildMemberAdd:
		if !s.TrackMembers {
			return
		}

		g := s.Guild(true, evt.GuildID)
		if g != nil {
			g.MemberAdd(true, evt.Member)
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
		s.ChannelAddUpdate(evt.Channel)
	case *discordgo.ChannelUpdate:
		s.ChannelAddUpdate(evt.Channel)
	case *discordgo.ChannelDelete:
		s.ChannelRemove(evt.Channel)

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
		if channel == nil {
			return
		}
		if channel.IsPrivate() && s.ThrowAwayDMMessages {
			return
		}

		channel.MessageAddUpdate(true, evt.Message, s.MaxChannelMessages, s.MaxMessageAge)
	case *discordgo.MessageUpdate:
		channel := s.Channel(true, evt.ChannelID)
		if channel == nil {
			return
		}
		if channel.IsPrivate() && s.ThrowAwayDMMessages {
			return
		}

		channel.MessageAddUpdate(true, evt.Message, s.MaxChannelMessages, s.MaxMessageAge)
	case *discordgo.MessageDelete:
		channel := s.Channel(true, evt.ChannelID)
		if channel == nil {
			return
		}
		if channel.IsPrivate() && s.ThrowAwayDMMessages {
			return
		}
		channel.MessageRemove(true, evt.Message.ID, s.RemoveDeletedMessages)
	case *discordgo.MessageDeleteBulk:
		channel := s.Channel(true, evt.ChannelID)
		if channel == nil {
			return
		}
		if channel.IsPrivate() && s.ThrowAwayDMMessages {
			return
		}
		channel.Owner.Lock()
		defer channel.Owner.Unlock()

		for _, v := range evt.Messages {
			channel.MessageRemove(false, v, s.RemoveDeletedMessages)
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
	case *discordgo.Ready:
		s.HandleReady(evt)
	default:
		handled = true
		return
	}

	handled = true

	if s.Debug {
		t := reflect.Indirect(reflect.ValueOf(i)).Type()
		log.Printf("Handled event %s; %#v", t.Name(), i)
	}
}

type RWLocker interface {
	RLock()
	RUnlock()
	Lock()
	Unlock()
}
