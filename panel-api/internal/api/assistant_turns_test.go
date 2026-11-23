package api

import (
	"testing"

	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// The model always gets user and assistant in turn, before the new question.
func TestPastTurns(t *testing.T) {
	m := func(roles ...string) []store.ConversationMessage {
		var out []store.ConversationMessage
		for _, r := range roles {
			out = append(out, store.ConversationMessage{Role: r, Text: r})
		}
		return out
	}
	for _, c := range []struct {
		in   []store.ConversationMessage
		want string
	}{
		{m(), ""},
		{m("user", "assistant"), "user,assistant"},
		{m("user", "user", "assistant", "assistant"), "user,assistant"}, // two tabs at once
		{m("assistant", "user", "assistant"), "user,assistant"},
		{m("user", "assistant", "user"), "user,assistant"},
	} {
		got := ""
		for i, turn := range pastTurns(c.in) {
			if i > 0 {
				got += ","
			}
			got += turn.Role
		}
		if got != c.want {
			t.Fatalf("%v: %s, want %s", c.in, got, c.want)
		}
	}
}
