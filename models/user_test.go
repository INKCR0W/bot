package models

import (
	"encoding/json"
	"testing"
)

func TestUser_SupportInlineQueries(t *testing.T) {
	var user User
	if err := json.Unmarshal([]byte(`{"id":42,"is_bot":true,"supports_inline_queries":true}`), &user); err != nil {
		t.Fatal(err)
	}
	if !user.SupportInlineQueries {
		t.Fatal("supports_inline_queries was not decoded")
	}
}

func TestUser_IgnoresArchiveSpelling(t *testing.T) {
	var user User
	if err := json.Unmarshal([]byte(`{"id":42,"is_bot":true,"support_inline_queries":true}`), &user); err != nil {
		t.Fatal(err)
	}
	if user.SupportInlineQueries {
		t.Fatal("accepted the SDK archive spelling in an unwrapped user")
	}
}
