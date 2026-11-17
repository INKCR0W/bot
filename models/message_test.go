package models

import (
	"encoding/json"
	"testing"
)

func TestUnmarshalMaybeInaccessibleMessage_inaccessible(t *testing.T) {
	src := `{"date":0,"chat":{"id":123},"message_id":987}`

	mim := MaybeInaccessibleMessage{}
	err := mim.UnmarshalJSON([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if mim.Type != MaybeInaccessibleMessageTypeInaccessibleMessage {
		t.Fatal("wrong type")
	}

	if mim.InaccessibleMessage == nil {
		t.Fatal("InaccessibleMessage is nil")
	}

	if mim.InaccessibleMessage.Chat.ID != 123 {
		t.Fatal("wrong chat id")
	}

	if mim.InaccessibleMessage.MessageID != 987 {
		t.Fatal("wrong message id")
	}
}

func TestUnmarshalMaybeInaccessibleMessage_message(t *testing.T) {
	src := `{"date":42,"chat":{"id":123},"message_id":987}`

	mim := MaybeInaccessibleMessage{}
	err := mim.UnmarshalJSON([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if mim.Type != MaybeInaccessibleMessageTypeMessage {
		t.Fatal("wrong type")
	}

	if mim.Message == nil {
		t.Fatal("Message is nil")
	}

	if mim.Message.Chat.ID != 123 {
		t.Fatal("wrong chat id")
	}

	if mim.Message.ID != 987 {
		t.Fatal("wrong message id")
	}
}

func TestCallbackQuery_MessageReplay(t *testing.T) {
	for _, tc := range []struct {
		name     string
		message  MaybeInaccessibleMessage
		wantChat int64
		wantID   int
	}{
		{name: "inline"},
		{
			name:     "accessible",
			message:  MaybeInaccessibleMessage{Message: &Message{ID: 9, Date: 42, Chat: Chat{ID: -100123}}},
			wantChat: -100123,
			wantID:   9,
		},
		{
			name: "inaccessible",
			message: MaybeInaccessibleMessage{
				Type:                MaybeInaccessibleMessageTypeInaccessibleMessage,
				InaccessibleMessage: &InaccessibleMessage{MessageID: 10, Chat: Chat{ID: -100456}},
			},
			wantChat: -100456,
			wantID:   10,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			update := &Update{CallbackQuery: &CallbackQuery{
				ID: "callback", From: User{ID: 12}, Message: tc.message,
			}}
			if tc.wantChat == 0 {
				update.CallbackQuery.InlineMessageID = "inline"
			}

			for _, archive := range []struct {
				name  string
				value any
			}{
				{name: "callback value", value: *update.CallbackQuery},
				{name: "callback pointer", value: update.CallbackQuery},
				{name: "update", value: update},
			} {
				t.Run(archive.name, func(t *testing.T) {
					wire, err := json.Marshal(archive.value)
					if err != nil {
						t.Fatal(err)
					}
					var replayed Update
					var target any = &replayed
					if _, isUpdate := archive.value.(*Update); !isUpdate {
						replayed.CallbackQuery = &CallbackQuery{}
						target = replayed.CallbackQuery
					}
					if err := json.Unmarshal(wire, target); err != nil {
						t.Fatal(err)
					}
					if replayed.CallbackQuery == nil || replayed.CallbackQuery.InlineMessageID != update.CallbackQuery.InlineMessageID {
						t.Fatalf("replayed callback = %+v", replayed.CallbackQuery)
					}
					checkReplayedCallbackMessage(t, &replayed, tc.wantChat, tc.wantID, tc.message.Type)
				})
			}
		})
	}
}

func TestCallbackQuery_LegacyMessageReplay(t *testing.T) {
	for _, tc := range []struct {
		name     string
		message  string
		wantChat int64
		wantID   int
		wantType MaybeInaccessibleMessageType
	}{
		{name: "inline", message: `{"Type":0,"Message":null,"InaccessibleMessage":null}`},
		{
			name:     "accessible",
			message:  `{"Type":0,"Message":{"date":42,"chat":{"id":-100123},"message_id":9},"InaccessibleMessage":null}`,
			wantChat: -100123,
			wantID:   9,
		},
		{
			name:     "inaccessible",
			message:  `{"Type":1,"Message":null,"InaccessibleMessage":{"date":0,"chat":{"id":-100456},"message_id":10}}`,
			wantChat: -100456,
			wantID:   10,
			wantType: MaybeInaccessibleMessageTypeInaccessibleMessage,
		},
		{
			name:     "wire object with extra fields",
			message:  `{"date":0,"chat":{"id":-100456},"message_id":10,"Type":"extra","Message":null,"InaccessibleMessage":null}`,
			wantChat: -100456,
			wantID:   10,
			wantType: MaybeInaccessibleMessageTypeInaccessibleMessage,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := &CallbackQuery{}
			src := `{"id":"callback","message":` + tc.message + `}`
			if err := json.Unmarshal([]byte(src), q); err != nil {
				t.Fatal(err)
			}
			checkReplayedCallbackMessage(t, &Update{CallbackQuery: q}, tc.wantChat, tc.wantID, tc.wantType)
		})
	}
}

func checkReplayedCallbackMessage(t *testing.T, update *Update, wantChat int64, wantID int, wantType MaybeInaccessibleMessageType) {
	t.Helper()
	chat := update.EffectiveChat()
	msg := update.CallbackQuery.Message
	if wantChat == 0 {
		if chat != nil || msg.Message != nil || msg.InaccessibleMessage != nil {
			t.Fatalf("inline callback unexpectedly has message %+v and chat %+v", msg, chat)
		}
		return
	}
	if chat == nil || chat.ID != wantChat {
		t.Fatalf("effective chat = %+v, want ID %d", chat, wantChat)
	}
	if msg.Type != wantType {
		t.Fatalf("message type = %d, want %d", msg.Type, wantType)
	}
	switch msg.Type {
	case MaybeInaccessibleMessageTypeMessage:
		if msg.Message != nil && msg.Message.ID == wantID && msg.InaccessibleMessage == nil {
			return
		}
	case MaybeInaccessibleMessageTypeInaccessibleMessage:
		if msg.InaccessibleMessage != nil && msg.InaccessibleMessage.MessageID == wantID && msg.Message == nil {
			return
		}
	}
	t.Fatalf("callback did not retain message %d: %+v", wantID, msg)
}

func TestMaybeInaccessibleMessage_MarshalValue(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value MaybeInaccessibleMessage
	}{
		{name: "zero value"},
		{
			name:  "accessible",
			value: MaybeInaccessibleMessage{Message: &Message{ID: 9, Date: 42, Chat: Chat{ID: -100123}}},
		},
		{
			name: "inaccessible",
			value: MaybeInaccessibleMessage{
				Type:                MaybeInaccessibleMessageTypeInaccessibleMessage,
				InaccessibleMessage: &InaccessibleMessage{MessageID: 10, Chat: Chat{ID: -100456}},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			want, err := json.Marshal(&tc.value)
			if err != nil {
				t.Fatal(err)
			}
			if string(wire) != string(want) {
				t.Fatalf("value = %s, pointer = %s", wire, want)
			}
		})
	}
	var absent *MaybeInaccessibleMessage
	if wire, err := json.Marshal(absent); err != nil || string(wire) != "null" {
		t.Fatalf("marshal nil = %s, %v, want null", wire, err)
	}
}

func TestUnmarshalMaybeInaccessibleMessage_ReusedDestination(t *testing.T) {
	msg := MaybeInaccessibleMessage{Message: &Message{ID: 9, Date: 42, Chat: Chat{ID: -100123}}}
	if err := json.Unmarshal([]byte(`{"date":0,"chat":{"id":-100456},"message_id":10}`), &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Message != nil || msg.InaccessibleMessage == nil || msg.InaccessibleMessage.MessageID != 10 {
		t.Fatalf("inaccessible update retained old message or lost new one: %+v", msg)
	}
	if err := json.Unmarshal([]byte(`{"date":42,"message_id":"invalid"}`), &msg); err == nil {
		t.Fatal("invalid message decoded unexpectedly")
	}
	if msg.Type != MaybeInaccessibleMessageTypeInaccessibleMessage ||
		msg.InaccessibleMessage == nil || msg.InaccessibleMessage.MessageID != 10 || msg.Message != nil {
		t.Fatalf("invalid data replaced the original message: %+v", msg)
	}
	if err := msg.UnmarshalJSON([]byte(" \nnull\t")); err != nil {
		t.Fatal(err)
	}
	if msg.Message != nil || msg.InaccessibleMessage != nil {
		t.Fatalf("null left a message: %+v", msg)
	}
}

func TestUnmarshalMessage_replyToStory(t *testing.T) {
	src := `{"date":0,"chat":{"id":123},"message_id":987,"reply_to_story":{"chat":{"id":42},"id":7}}`

	msg := Message{}
	if err := json.Unmarshal([]byte(src), &msg); err != nil {
		t.Fatal(err)
	}

	if msg.ReplyToStory == nil {
		t.Fatal("ReplyToStory is nil")
	}

	if msg.ReplyToStory.ID != 7 {
		t.Fatal("wrong story id")
	}

	if msg.ReplyToStory.Chat.ID != 42 {
		t.Fatal("wrong story chat id")
	}
}
