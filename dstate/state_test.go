package dstate

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"testing"
)

var testState *State

func init() {
	testState = NewState()
	testGuild := createTestGuild("0", "01")
	testState.GuildCreate(false, testGuild)
}

func createTestGuild(gID, cID string) *discordgo.Guild {
	return &discordgo.Guild{
		ID:   gID,
		Name: gID,
		Channels: []*discordgo.Channel{
			&discordgo.Channel{ID: cID, Name: cID},
		},
	}
}

func createTestMessage(mID, cID, content string) *discordgo.Message {
	return &discordgo.Message{ID: mID, Content: content, ChannelID: cID}
}

func genStringIdMap(num int) []string {
	out := make([]string, num)
	for i := 0; i < num; i++ {
		out[i] = fmt.Sprint(i)
	}
	return out
}

func TestGuildCreate(t *testing.T) {
	g := createTestGuild("testguild", "testchan")
	s := NewState()
	s.GuildCreate(true, g)

	// Check if guild got added
	gs := s.Guild(true, "testguild")
	if gs == nil {
		t.Fatal("GuildState is nil")
	}

	// Check if channel got added
	cs := s.Channel(true, "testchan")
	if cs == nil {
		t.Fatal("ChannelState is nil in global map")
	}

	cs = gs.Channel(true, "testchan")
	if cs == nil {
		t.Fatal("ChannelState is nil in guildstate map")
	}
}

func TestSecondReady(t *testing.T) {
	r := &discordgo.Ready{
		Guilds: []*discordgo.Guild{
			&discordgo.Guild{
				ID:          "1",
				Name:        "G 1",
				Unavailable: true,
			},
		},
	}

	s := NewState()
	s.HandleReady(r)

	loadGuilds := func() {
		for _, v := range r.Guilds {
			g := &discordgo.Guild{
				ID:   v.ID,
				Name: v.Name,
				Channels: []*discordgo.Channel{
					&discordgo.Channel{
						ID:   v.ID + "01",
						Name: v.ID + "01",
					},
					&discordgo.Channel{
						ID:   v.ID + "02",
						Name: v.ID + "02",
					},
					&discordgo.Channel{
						ID:   v.ID + "03",
						Name: v.ID + "03",
					},
				},
			}
			s.GuildCreate(true, g)
		}
	}

	loadGuilds()

	doChecks := func(prefix string) {
		gs := s.Guild(true, "1")
		if gs == nil {
			t.Fatal(prefix + " GuildState is nil")
		}

		csLocal := gs.Channel(true, "101")
		if csLocal == nil {
			t.Fatal(prefix + " csLocal == nil")
		}

		csGlobal := s.Channel(true, "101")
		if csGlobal == nil {
			t.Fatal(prefix + " csGlobal == nil")
		}
	}
	doChecks("Initial:")

	s.HandleReady(r)

	doChecks("SecondReady:")

	loadGuilds()

	doChecks("SecondLoad:")
}

func TestGuildDelete(t *testing.T) {
	s := NewState()
	g := createTestGuild("testguild", "testchan")
	s.GuildCreate(true, g)

	s.GuildRemove("testguild")

	// Check if guild got removed
	gs := s.Guild(true, "testguild")
	if gs != nil {
		t.Fatal("GuildState is not nil")
	}

	// Check if channel got removed
	cs := s.Channel(true, "testchan")
	if cs != nil {
		t.Fatal("ChannelState is not nil in global map")
	}
}

func TestMessageCreate(t *testing.T) {
	s := NewState()
	s.MaxChannelMessages = 100
	g := createTestGuild("testguild", "testchan")
	s.GuildCreate(true, g)

	msgEvt1 := &discordgo.MessageCreate{
		Message: createTestMessage("a", "testchan", "Hello there buddy"),
	}
	msgEvt2 := &discordgo.MessageCreate{
		Message: createTestMessage("b", "testchan", "Hello there buddy"),
	}

	cs := s.Channel(true, "testchan")
	if cs == nil {
		t.Fatal("ChannelState is nil")
	}

	s.HandleEvent(nil, msgEvt1)
	s.HandleEvent(nil, msgEvt2)

	if len(cs.Messages) != 2 {
		t.Fatal("Length of messages not 4:", cs.Messages)
	}

	for i := 0; i < 150; i++ {
		s.HandleEvent(nil, &discordgo.MessageCreate{
			Message: createTestMessage(fmt.Sprint(i), "testchan", "HHeyyy"),
		})
	}

	if len(cs.Messages) != 100 {
		t.Fatal("Length of messages not 100:", len(cs.Messages))
	}
}

func BenchmarkMessageCreate(b *testing.B) {
	s := NewState()
	s.MaxChannelMessages = 100

	g := createTestGuild("testguild", "testchan")
	s.GuildCreate(true, g)

	idMap := genStringIdMap(b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msgEvt := &discordgo.MessageCreate{
			Message: createTestMessage(idMap[i], "testchan", "Hello there buddy"),
		}

		s.HandleEvent(nil, msgEvt)
	}
}

func BenchmarkMessageCreateParallel(b *testing.B) {
	s := NewState()
	s.MaxChannelMessages = 100

	g := createTestGuild("testguild", "testchan")
	s.GuildCreate(true, g)

	idMap := genStringIdMap(b.N)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			msgEvt := &discordgo.MessageCreate{
				Message: createTestMessage(idMap[i], "testchan", "Hello there buddy"),
			}
			s.HandleEvent(nil, msgEvt)
			i++
		}
	})
}

func BenchmarkMessageCreateParallelMultiGuild100(b *testing.B) {
	s := NewState()
	s.MaxChannelMessages = 100

	for i := 0; i < 100; i++ {
		g := createTestGuild("g"+fmt.Sprint(i), fmt.Sprint(i))
		s.GuildCreate(true, g)
	}

	idMap := genStringIdMap(b.N)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			msgEvt := &discordgo.MessageCreate{
				Message: createTestMessage(idMap[i], idMap[i%100], "Hello there buddy"),
			}
			s.HandleEvent(nil, msgEvt)
			i++
		}
	})
}

// func BenchmarkDGOStateMessageCreatePalellMultiGuild100(b *testing.B) {
// 	s := discordgo.NewState()
// 	s.MaxMessageCount = 100

// 	for i := 0; i < 100; i++ {
// 		g := &discordgo.Guild{
// 			ID: fmt.Sprintf("g%d", i),
// 			Channels: []*discordgo.Channel{
// 				&discordgo.Channel{ID: fmt.Sprint(i), Name: fmt.Sprint(i)},
// 			},
// 		}
// 		s.OnInterface(nil, &discordgo.GuildCreate{g})
// 	}

// 	idMap := genStringIdMap(b.N)

// 	b.ResetTimer()

// 	b.RunParallel(func(pb *testing.PB) {
// 		i := 0
// 		for pb.Next() {
// 			msgEvt := &discordgo.MessageCreate{
// 				Message: createTestMessage(idMap[i], idMap[i%100], "Hello there buddy"),
// 			}
// 			s.OnInterface(nil, msgEvt)
// 			i++
// 		}
// 	})
// }
