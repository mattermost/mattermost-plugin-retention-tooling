package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseNamedArgs(t *testing.T) {
	data := []struct {
		name string
		s    string
		m    map[string]string
	}{
		{"empty", "", map[string]string{}},
		{"gibberish", "ifu3ue-h29f8", map[string]string{}},
		{"action only", "channel-archiver status", map[string]string{SubCommandKey: "status"}},
		{"no action", "channel-archiver --arg1 val1 --arg2 val2", map[string]string{"arg1": "val1", "arg2": "val2"}},
		{"command only", "channel-archiver", map[string]string{}},
		{"trailing empty arg", "channel-archiver add --arg1 val1 --arg2", map[string]string{SubCommandKey: "add", "arg1": "val1", "arg2": ""}},
		{"leading empty arg", "channel-archiver add --arg1 --arg2 val2", map[string]string{SubCommandKey: "add", "arg1": "", "arg2": "val2"}},
		{"weird", "-- -- -- --", map[string]string{}},
		{"hyphen before action", "channel-archiver -- add", map[string]string{}},
		{"trailing hyphen", "channel-archiver add -- ", map[string]string{SubCommandKey: "add"}},
		{"hyphen in val", "channel-archiver add --arg1 val-1 ", map[string]string{SubCommandKey: "add", "arg1": "val-1"}},
		{"quote prefix and suffix", "channel-archiver add --arg1 \"val-1\"", map[string]string{SubCommandKey: "add", "arg1": "val-1"}},
		{"quote embedded", "channel-archiver add --arg1 O'Brien", map[string]string{SubCommandKey: "add", "arg1": "O'Brien"}},
		{"quote prefix, suffix, and embedded", "channel-archiver add --arg1 \"O'Brien\"", map[string]string{SubCommandKey: "add", "arg1": "O'Brien"}},
		{"empty quotes", "channel-archiver add --arg1 \"\"", map[string]string{SubCommandKey: "add", "arg1": ""}},
	}

	for _, tt := range data {
		m := parseNamedArgs(tt.s)
		assert.NotNil(t, m)
		assert.Equal(t, tt.m, m, tt.name)
	}
}
