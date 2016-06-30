package dutil

import (
	"github.com/bwmarrin/discordgo"
	"strings"
	"unicode"
	"unicode/utf8"
)

// A helper for sending potentially long messages
// If the message is longer than 2k characters it will split at
// Last newline before 2k or last whitespace before 2k or if that fails
// (no whitespace) just split at 2k
func SplitSendMessage(s *discordgo.Session, channelID, message string) ([]*discordgo.Message, error) {

	msg, rest := StrSplit(message, 2000)
	discordMessage, err := s.ChannelMessageSend(channelID, msg)
	if err != nil {
		return nil, err
	}

	ret := []*discordgo.Message{discordMessage}

	if rest != "" {
		m, err := SplitSendMessage(s, channelID, rest)
		if err != nil {
			return ret, err
		}
		ret = append(ret, m...)
	}

	return ret, nil
}

// Will split "s" before runecount at last possible newline, whitespace or just at "runecount" if there is no whitespace
// If the runecount in "s" is less than "runeCount" then "last" will be zero
func StrSplit(s string, runeCount int) (split, rest string) {
	// Possibly split up message
	if utf8.RuneCountInString(s) > runeCount {
		_, beforeIndex := RuneByIndex(s, runeCount)
		firstPart := s[:beforeIndex]

		// Split at newline if possible
		foundWhiteSpace := false
		lastIndex := strings.LastIndex(firstPart, "\n")
		if lastIndex == -1 {
			// No newline, check for any possible whitespace then
			lastIndex = strings.LastIndexFunc(firstPart, func(r rune) bool {
				return unicode.In(r, unicode.White_Space)
			})
			if lastIndex == -1 {
				lastIndex = beforeIndex
			} else {
				foundWhiteSpace = true
			}
		} else {
			foundWhiteSpace = true
		}

		// Remove the whitespace we split at if any
		if foundWhiteSpace {
			_, rLen := utf8.DecodeRuneInString(s[lastIndex:])
			rest = s[lastIndex+rLen:]
		} else {
			rest = s[lastIndex:]
		}

		split = s[:lastIndex]
	} else {
		split = s
	}

	return
}

// Returns the string index from the rune position
// Panics if utf8.RuneCountInString(s) <= runeIndex or runePos < 0
func RuneByIndex(s string, runePos int) (rune, int) {
	sLen := utf8.RuneCountInString(s)
	if sLen <= runePos || runePos < 0 {
		panic("runePos is out of bounds")
	}

	i := 0
	last := rune(0)
	for k, r := range s {
		if i == runePos {
			return r, k
		}
		i++
		last = r
	}
	return last, i
}
