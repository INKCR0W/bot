package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInputPollOptionMarshalJSON_embedsInputMediaType(t *testing.T) {
	opt := InputPollOption{
		Text: "opt",
		Media: &InputMediaPhoto{
			Media: "AgACAgIAAxkBAAIC",
		},
	}
	b, err := json.Marshal(opt)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"type":"photo"`) {
		t.Fatalf("expected type photo in JSON: %s", s)
	}
}

func TestInputPollOptionMarshalJSON_omitsTypedNilMedia(t *testing.T) {
	var media *InputMediaPhoto
	b, err := json.Marshal(InputPollOption{Text: "option", Media: media})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "{\"text\":\"option\"}" {
		t.Fatalf("got %s, want only the option text", b)
	}
}

func TestInputPollOptionMarshalJSON_Link(t *testing.T) {
	const want = `{"text":"option","media":{"type":"link","url":"https://example.com/poll-option"}}`
	option := InputPollOption{
		Text:  "option",
		Media: &InputMediaLink{URL: "https://example.com/poll-option"},
	}
	b, err := json.Marshal(option)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != want {
		t.Fatalf("got %s, want %s", b, want)
	}
}

func TestPollMediaRoundTrip_Link(t *testing.T) {
	const want = "{\"link\":{\"url\":\"https://example.com/poll-option\"}}"
	var media PollMedia
	if err := json.Unmarshal([]byte(want), &media); err != nil {
		t.Fatal(err)
	}
	if media.Link == nil || media.Link.URL != "https://example.com/poll-option" {
		t.Fatalf("link unavailable after decoding: %#v", media.Link)
	}
	b, err := json.Marshal(media)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != want {
		t.Fatalf("got %s, want %s", b, want)
	}
}
