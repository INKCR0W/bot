package models

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestInlineKeyboardButton_Disabled verifies a disabled button is encoded as an
// empty object, which is how Telegram represents DisabledButton.
func TestInlineKeyboardButton_Disabled(t *testing.T) {
	out, err := json.Marshal(InlineKeyboardButton{Text: "Off", Disabled: &DisabledButton{}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `{"text":"Off","disabled":{}}`; string(out) != want {
		t.Fatalf("got %s, want %s", out, want)
	}
}

// TestInlineKeyboardMarkup_ForceReply verifies the 10.3 force_reply flag is sent
// only when set.
func TestInlineKeyboardMarkup_ForceReply(t *testing.T) {
	out, err := json.Marshal(InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{}, ForceReply: true})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `"force_reply":true`) {
		t.Fatalf("missing force_reply in %s", out)
	}

	out, _ = json.Marshal(InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{}})
	if strings.Contains(string(out), `"force_reply"`) {
		t.Fatalf("force_reply should be omitted when false: %s", out)
	}
}

// TestReplyKeyboardMarkup_ForceReply verifies the 10.3 force_reply flag is sent
// only when set.
func TestReplyKeyboardMarkup_ForceReply(t *testing.T) {
	out, err := json.Marshal(ReplyKeyboardMarkup{Keyboard: [][]KeyboardButton{}, ForceReply: true})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `"force_reply":true`) {
		t.Fatalf("missing force_reply in %s", out)
	}

	out, _ = json.Marshal(ReplyKeyboardMarkup{Keyboard: [][]KeyboardButton{}})
	if strings.Contains(string(out), `"force_reply"`) {
		t.Fatalf("force_reply should be omitted when false: %s", out)
	}
}

func TestSelectionFilters_EmbeddedJSONRetainsSiblingFields(t *testing.T) {
	for _, tt := range []struct {
		name   string
		data   string
		filter string
		model  any
	}{
		{
			name:   "users",
			data:   `{"label":"staff","request_id":1,"user_is_bot":false}`,
			filter: "user_is_bot",
			model: &struct {
				KeyboardButtonRequestUsers
				Label string `json:"label"`
			}{},
		},
		{
			name:   "chat",
			data:   `{"label":"staff","request_id":2,"chat_is_channel":false,"chat_is_forum":false}`,
			filter: "chat_is_forum",
			model: &struct {
				KeyboardButtonRequestChat
				Label string `json:"label"`
			}{},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(tt.data), tt.model); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(tt.model)
			if err != nil {
				t.Fatal(err)
			}
			var wire map[string]json.RawMessage
			if err := json.Unmarshal(data, &wire); err != nil {
				t.Fatal(err)
			}
			if string(wire["label"]) != `"staff"` || string(wire[tt.filter]) != "false" {
				t.Fatalf("expected embedded filter and outer label, got %s", data)
			}
		})
	}
}
