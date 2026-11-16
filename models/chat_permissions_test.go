package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestChatPermissions_JSON_encodesFalseCanSendMessages(t *testing.T) {
	p := ChatPermissions{
		CanSendMessages:      false,
		CanSendPhotos:        true,
		CanSendVideos:        true,
		CanSendOtherMessages: true,
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"can_send_messages":false`) {
		t.Fatalf("expected explicit false for can_send_messages, got %s", s)
	}
}

func TestChatPermissions_JSON_encodesFalseInheritedPermissions(t *testing.T) {
	permissions := ChatPermissions{CanPinMessages: true, CanSendMessages: true}
	data, err := json.Marshal(permissions)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]bool
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"can_manage_topics", "can_edit_tag", "can_react_to_messages"} {
		if allowed, present := fields[name]; allowed || !present {
			t.Errorf("expected %s to be explicitly false, got %s", name, data)
		}
	}
}
