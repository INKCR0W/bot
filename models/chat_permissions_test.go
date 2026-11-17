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

func TestChatPermissions_JSON_roundTripsOptionalPermissions(t *testing.T) {
	var permissions ChatPermissions
	if err := json.Unmarshal([]byte(`{"can_send_messages":true,"can_pin_messages":true,"can_edit_tag":false,"can_react_to_messages":true}`), &permissions); err != nil {
		t.Fatal(err)
	}
	if permissions.CanManageTopics != nil {
		t.Fatal("expected omitted can_manage_topics to decode as nil")
	}
	if permissions.CanEditTag == nil || *permissions.CanEditTag {
		t.Fatal("expected can_edit_tag to decode as explicit false")
	}
	if permissions.CanReactToMessages == nil || !*permissions.CanReactToMessages {
		t.Fatal("expected can_react_to_messages to decode as explicit true")
	}

	data, err := json.Marshal(permissions)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]bool
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if _, present := fields["can_manage_topics"]; present {
		t.Errorf("expected can_manage_topics to be omitted, got %s", data)
	}
	if allowed, present := fields["can_edit_tag"]; allowed || !present {
		t.Errorf("expected can_edit_tag to be explicitly false, got %s", data)
	}
	if !fields["can_react_to_messages"] {
		t.Errorf("expected can_react_to_messages to be explicitly true, got %s", data)
	}
}
