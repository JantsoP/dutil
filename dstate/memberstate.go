package dstate

import (
	"encoding/hex"
	"github.com/jonas747/discordgo"
	"strconv"
	"strings"
	"time"
)

// MemberState represents the state of a member
type MemberState struct {
	Guild *GuildState

	// The ID of the member, safe to access without locking
	ID int64 `json:"id"`

	MemberSet bool `json:"member_set"`

	// The time at which the member joined the guild, in ISO8601.
	// This may be zero if the member hasnt been updated
	JoinedAt time.Time `json:"joined_at"`

	// The nickname of the member, if they have one.
	Nick string `json:"nick"`

	// A list of IDs of the roles which are possessed by the member.
	Roles []int64 `json:"roles"`

	// Wether the presence Information was set
	PresenceSet    bool             `json:"presence_set"`
	PresenceStatus discordgo.Status `json:"presence_status"`
	PresenceGame   *discordgo.Game  `json:"presence_game"`

	// The users username.
	Username string `json:"username"`

	// The hash of the user's avatar. Use Session.UserAvatar
	// to retrieve the avatar itself.
	Avatar         [16]byte `json:"avatar"`
	AnimatedAvatar bool

	// The discriminator of the user (4 numbers after name).
	Discriminator int32 `json:"discriminator"`

	// Whether the user is a bot, safe to access without locking
	Bot bool `json:"bot"`
}

// StrID is the same as above, formatted as a string
func (m *MemberState) StrID() string {
	return discordgo.StrID(m.ID)
}

func (m *MemberState) UpdateMember(member *discordgo.Member) {
	// Patch
	if member.JoinedAt != "" {
		m.JoinedAt, _ = time.Parse("2006-01-02T15:04:05-0700", member.JoinedAt)
	}

	if member.Roles != nil {
		m.Roles = member.Roles
	}

	// Seems to always be provided
	m.Nick = member.Nick

	m.Username = member.User.Username
	m.parseAvatar(member.User.Avatar)

	discrim, _ := strconv.ParseInt(member.User.Discriminator, 10, 32)
	m.Discriminator = int32(discrim)

	m.MemberSet = true
}

func (m *MemberState) UpdatePresence(presence *discordgo.Presence) {
	m.PresenceSet = true
	m.PresenceStatus = presence.Status
	m.PresenceGame = presence.Game

	if !m.MemberSet {
		m.Nick = presence.Nick
	}

	if presence.User.Username != "" {
		m.Username = presence.User.Username
	}

	if presence.User.Discriminator != "" {
		discrim, _ := strconv.ParseInt(presence.User.Discriminator, 10, 32)
		m.Discriminator = int32(discrim)
	}

	if presence.User.Avatar != "" {
		m.parseAvatar(presence.User.Avatar)
	}
}

func (m *MemberState) parseAvatar(str string) {
	if strings.HasPrefix(str, "a_") {
		str = str[2:]
		m.AnimatedAvatar = true
	} else {
		m.AnimatedAvatar = false
	}

	hex.Decode(m.Avatar[:], []byte(str))
}

// Copy returns a copy of the state, this is not a deep copy so the slices will point to the same arrays, so they're only read safe, not write safe
func (m *MemberState) Copy() *MemberState {
	cop := new(MemberState)
	*cop = *m
	return cop
}
