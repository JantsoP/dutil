package dutil

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSplitSendMessage(t *testing.T) {

}

type StrSplitTestCase struct {
	Str      string
	ExpFirst string
	ExpRest  string
}

func TestSplitStr(t *testing.T) {
	cases := []StrSplitTestCase{
		StrSplitTestCase{"123456789", "1234", "56789"},   // #0
		StrSplitTestCase{"123\n456789", "123", "456789"}, // #1
		StrSplitTestCase{"123 456789", "123", "456789"},  // #2
		StrSplitTestCase{"1234", "1234", ""},             // #3
		StrSplitTestCase{"123", "123", ""},               // #4
		StrSplitTestCase{"12345", "1234", "5"},           // #5
		StrSplitTestCase{"123 ", "123 ", ""},             // #6
		StrSplitTestCase{"123 4", "123", "4"},            // #7
		StrSplitTestCase{"123  ", "123", " "},            // #8
	}

	for k, c := range cases {
		first, last := StrSplit(c.Str, 4)
		assert.Equal(t, c.ExpFirst, first, "case #%d", k)
		assert.Equal(t, c.ExpRest, last, "case #%d", k)
	}
}
