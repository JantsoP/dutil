package dutil

// Package dutil provides general discordgo utilities that i find to be reusing across my discord projects

import (
	"github.com/bwmarrin/discordgo"
)

// Need mah pr to go through
// Returns all guild members in a guild
// It will make `number of members`/1000 requests to the api
// func GetAllGuildMembers(session *discordgo.Session, guilID string) ([]*discordgo.Member, error) {
// 	var after string
// 	members := make([]*discordgo.Member, 0)

// 	for {
// 		resp, err := session.GuildMembers(guilID, after, 1000)
// 		if err != nil {
// 			return nil, err
// 		}
// 		members = append(members, resp...)

// 		if len(resp) < 1000 {
// 			break // Reached the end
// 		}

// 		after = members[len(members)-1].User.ID
// 	}
// 	return members, nil
// }
