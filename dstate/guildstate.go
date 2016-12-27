package dstate

import (
	"errors"
	"github.com/jonas747/discordgo"
	"sync"
	"time"
)

var (
	ErrMemberNotFound  = errors.New("Member not found")
	ErrChannelNotFound = errors.New("Channel not found")
)

type GuildState struct {
	sync.RWMutex

	// The underlying guild, the members and channels fields shouldnt be used
	Guild *discordgo.Guild

	Members  map[string]*MemberState
	Channels map[string]*ChannelState

	maxMessages           int           // Absolute max number of messages cached in a channel
	maxMessageDuration    time.Duration // Max age of messages, if 0 ignored. (Only checks age whena new message is received on the channel)
	removeDeletedMessages bool
	removeOfflineMembers  bool
}

// NewGuildstate creates a new guild state, it only uses the passed state to get settings from
// Pass nil to use default settings
func NewGuildState(guild *discordgo.Guild, state *State) *GuildState {
	guildState := &GuildState{
		Guild:    guild,
		Members:  make(map[string]*MemberState),
		Channels: make(map[string]*ChannelState),
	}

	if state != nil {
		guildState.maxMessages = state.MaxChannelMessages
		guildState.maxMessageDuration = state.MaxMessageAge
		guildState.removeDeletedMessages = state.RemoveDeletedMessages
		guildState.removeOfflineMembers = state.RemoveOfflineMembers
	}

	for _, channel := range guild.Channels {
		guildState.ChannelAddUpdate(false, channel)
	}

	for _, member := range guild.Members {
		guildState.MemberAddUpdate(false, member)
	}

	for _, presence := range guild.Presences {
		guildState.PresenceAddUpdate(false, presence)
	}

	return guildState
}

func (g *GuildState) GuildUpdate(lock bool, newGuild *discordgo.Guild) {
	if lock {
		g.Lock()
		defer g.Unlock()
	}

	if newGuild.Roles == nil {
		newGuild.Roles = g.Guild.Roles
	}
	if newGuild.Emojis == nil {
		newGuild.Emojis = g.Guild.Emojis
	}
	if newGuild.Members == nil {
		newGuild.Members = g.Guild.Members
	}
	if newGuild.Presences == nil {
		newGuild.Presences = g.Guild.Presences
	}
	if newGuild.Channels == nil {
		newGuild.Channels = g.Guild.Channels
	}
	if newGuild.VoiceStates == nil {
		newGuild.VoiceStates = g.Guild.VoiceStates
	}

	*g.Guild = *newGuild
}

// Member returns a the member from an id, or nil if not found
func (g *GuildState) Member(lock bool, id string) *MemberState {
	if lock {
		g.RLock()
		defer g.RUnlock()
	}

	return g.Members[id]
}

// MemberCopy returns a full copy of a member, or nil if not found
// If light is true, roles will not be copied
func (g *GuildState) MemberCopy(lock bool, id string, light bool) *discordgo.Member {
	if lock {
		g.RLock()
		defer g.RUnlock()
	}

	m := g.Member(false, id)
	if m == nil || m.Member == nil {
		return nil
	}

	mCopy := new(discordgo.Member)

	*mCopy = *m.Member
	mCopy.Roles = nil
	if !light {
		for _, r := range m.Member.Roles {
			mCopy.Roles = append(mCopy.Roles, r)
		}
	}
	return mCopy
}

// ChannelCopy returns a full copy of a member, or nil if not found
// if ligh is true, permissionoverwrites will not be copied
func (g *GuildState) ChannelCopy(lock bool, id string, light bool) *discordgo.Channel {
	if lock {
		g.RLock()
		defer g.RUnlock()
	}

	c := g.Channel(false, id)
	if c == nil || c.Channel == nil {
		return nil
	}

	cCopy := new(discordgo.Channel)
	*cCopy = *c.Channel

	cCopy.Messages = nil
	cCopy.PermissionOverwrites = nil

	if !light {
		for _, pow := range c.Channel.PermissionOverwrites {
			powCopy := new(discordgo.PermissionOverwrite)
			*powCopy = *pow
			cCopy.PermissionOverwrites = append(cCopy.PermissionOverwrites, pow)
		}
	}

	return cCopy
}

func (g *GuildState) MemberAddUpdate(lock bool, newMember *discordgo.Member) {
	if lock {
		g.Lock()
		defer g.Unlock()
	}

	existing, ok := g.Members[newMember.User.ID]
	if ok {
		if existing.Member == nil {
			existing.Member = newMember
		} else {
			// Patch
			if newMember.JoinedAt != "" {
				existing.Member.JoinedAt = newMember.JoinedAt
			}
			if newMember.Roles != nil {
				existing.Member.Roles = newMember.Roles
			}

			// Seems to always be provided
			existing.Member.Nick = newMember.Nick
			existing.Member.User = newMember.User
		}
	} else {
		g.Members[newMember.User.ID] = &MemberState{
			Member: newMember,
		}
	}
}

func (g *GuildState) MemberRemove(lock bool, id string) {
	if lock {
		g.Lock()
		defer g.Unlock()
	}
	delete(g.Members, id)
}

func (g *GuildState) PresenceAddUpdate(lock bool, newPresence *discordgo.Presence) {
	if lock {
		g.Lock()
		defer g.Unlock()
	}

	existing, ok := g.Members[newPresence.User.ID]
	if ok {
		if existing.Presence == nil {
			existing.Presence = newPresence
		} else {
			// Patch

			// Nil games indicates them not playing anything, so this had to always be provided?
			// IDK the docs dosen't seem to correspond to the actual results very well
			existing.Presence.Game = newPresence.Game

			if newPresence.Status != "" {
				existing.Presence.Status = newPresence.Status
			}
		}
	} else {
		g.Members[newPresence.User.ID] = &MemberState{
			Presence: newPresence,
		}
	}

	if newPresence.Status == discordgo.StatusOffline && g.removeOfflineMembers {
		// Remove after a minute incase they just restart the client or something
		time.AfterFunc(time.Minute, func() {
			g.Lock()
			defer g.Unlock()

			member := g.Member(false, newPresence.User.ID)
			if member != nil {
				if member.Presence == nil || member.Presence.Status == discordgo.StatusOffline {
					g.MemberRemove(false, newPresence.User.ID)
				}
			}
		})
	}
}

func (g *GuildState) Channel(lock bool, id string) *ChannelState {
	if lock {
		g.RLock()
		defer g.RUnlock()
	}

	return g.Channels[id]
}

func (g *GuildState) ChannelAddUpdate(lock bool, newChannel *discordgo.Channel) *ChannelState {
	if lock {
		g.Lock()
		defer g.Unlock()
	}

	existing, ok := g.Channels[newChannel.ID]
	if ok {
		// Patch
		if newChannel.PermissionOverwrites == nil {
			newChannel.PermissionOverwrites = existing.Channel.PermissionOverwrites
		}
		if newChannel.IsPrivate && newChannel.Recipient == nil {
			newChannel.Recipient = existing.Channel.Recipient
		}
		*existing.Channel = *newChannel
		return existing
	}

	state := &ChannelState{
		Channel:  newChannel,
		Messages: make([]*MessageState, 0),
		Guild:    g,
	}

	g.Channels[newChannel.ID] = state
	return state
}

func (g *GuildState) ChannelRemove(lock bool, id string) {
	if lock {
		g.Lock()
		defer g.Unlock()
	}
	delete(g.Channels, id)
}

func (g *GuildState) Role(lock bool, id string) *discordgo.Role {
	if lock {
		g.RLock()
		defer g.RUnlock()
	}

	for _, role := range g.Guild.Roles {
		if role.ID == id {
			return role
		}
	}

	return nil
}

func (g *GuildState) RoleAddUpdate(lock bool, newRole *discordgo.Role) {
	if lock {
		g.Lock()
		defer g.Unlock()
	}

	existing := g.Role(false, newRole.ID)
	if existing != nil {
		*existing = *newRole
	} else {
		g.Guild.Roles = append(g.Guild.Roles, newRole)
	}
}

func (g *GuildState) RoleRemove(lock bool, id string) {
	if lock {
		g.Lock()
		defer g.Unlock()
	}

	for i, v := range g.Guild.Roles {
		if v.ID == id {
			g.Guild.Roles = append(g.Guild.Roles[:i], g.Guild.Roles[i+1:]...)
			return
		}
	}
}

func (g *GuildState) VoiceState(lock bool, userID string) *discordgo.VoiceState {
	if lock {
		g.RLock()
		defer g.RUnlock()
	}

	for _, v := range g.Guild.VoiceStates {
		if v.UserID == userID {
			return v
		}
	}

	return nil
}

func (g *GuildState) VoiceStateUpdate(lock bool, update *discordgo.VoiceStateUpdate) {
	if lock {
		g.Lock()
		defer g.Unlock()
	}

	// Handle Leaving Channel
	if update.ChannelID == "" {
		for i, state := range g.Guild.VoiceStates {
			if state.UserID == update.UserID {
				g.Guild.VoiceStates = append(g.Guild.VoiceStates[:i], g.Guild.VoiceStates[i+1:]...)
				return
			}
		}
	}

	existing := g.VoiceState(false, update.UserID)
	if existing != nil {
		*existing = *update.VoiceState
		return
	}

	g.Guild.VoiceStates = append(g.Guild.VoiceStates, update.VoiceState)

	return
}

// Calculates the permissions for a member.
// https://support.discordapp.com/hc/en-us/articles/206141927-How-is-the-permission-hierarchy-structured-
func (g *GuildState) MemberPermissions(lock bool, channelID string, memberID string) (apermissions int, err error) {
	if lock {
		g.Lock()
		defer g.Unlock()
	}

	if memberID == g.Guild.OwnerID {
		return discordgo.PermissionAll, nil
	}

	mState := g.Member(false, memberID)
	if mState == nil || mState.Member == nil {
		return 0, ErrMemberNotFound
	}

	for _, role := range g.Guild.Roles {
		if role.ID == g.Guild.ID {
			apermissions |= role.Permissions
			break
		}
	}

	for _, role := range g.Guild.Roles {
		for _, roleID := range mState.Member.Roles {
			if role.ID == roleID {
				apermissions |= role.Permissions
				break
			}
		}
	}

	// Administrator bypasses channel overrides
	if apermissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator {
		apermissions |= discordgo.PermissionAll
		return
	}

	cState := g.Channel(false, channelID)
	if cState == nil {
		err = ErrChannelNotFound
		return
	}

	// Apply @everyone overrides from the channel.
	for _, overwrite := range cState.Channel.PermissionOverwrites {
		if g.Guild.ID == overwrite.ID {
			apermissions &= ^overwrite.Deny
			apermissions |= overwrite.Allow
			break
		}
	}

	denies := 0
	allows := 0

	// Member overwrites can override role overrides, so do two passes
	for _, overwrite := range cState.Channel.PermissionOverwrites {
		for _, roleID := range mState.Member.Roles {
			if overwrite.Type == "role" && roleID == overwrite.ID {
				denies |= overwrite.Deny
				allows |= overwrite.Allow
				break
			}
		}
	}

	apermissions &= ^denies
	apermissions |= allows

	for _, overwrite := range cState.Channel.PermissionOverwrites {
		if overwrite.Type == "member" && overwrite.ID == memberID {
			apermissions &= ^overwrite.Deny
			apermissions |= overwrite.Allow
			break
		}
	}

	if apermissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator {
		apermissions |= discordgo.PermissionAllChannel
	}

	return
}
