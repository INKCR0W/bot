package models

import (
	"encoding/json"
	"testing"
)

func TestForumTopicEditedIconSurvivesRoundTrip(t *testing.T) {
	empty, icon := "", "5368324170671202286"
	for _, tc := range []struct {
		name string
		json string
		icon *string
	}{
		{name: "unchanged", json: `{"name":"Updated topic"}`},
		{name: "removed", json: `{"icon_custom_emoji_id":""}`, icon: &empty},
		{name: "replaced", json: `{"icon_custom_emoji_id":"5368324170671202286"}`, icon: &icon},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var update Update
			body := `{"update_id":1,"message":{"message_id":2,"chat":{"id":-100123,"type":"supergroup"},"forum_topic_edited":` + tc.json + `}}`
			if err := json.Unmarshal([]byte(body), &update); err != nil {
				t.Fatal(err)
			}
			if update.Message == nil || update.Message.ForumTopicEdited == nil {
				t.Fatalf("missing edited topic in update %+v", update)
			}
			edit := *update.Message.ForumTopicEdited
			got := edit.IconCustomEmojiID
			if (tc.icon == nil && got != nil) || (tc.icon != nil && (got == nil || *got != *tc.icon)) {
				t.Fatalf("unexpected icon for %s: %+v", tc.json, edit)
			}
			encoded, err := json.Marshal(edit)
			if err != nil {
				t.Fatal(err)
			}
			if string(encoded) != tc.json {
				t.Fatalf("encoded edit = %s, want %s", encoded, tc.json)
			}
		})
	}
}
