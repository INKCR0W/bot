package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type keyboardCall struct {
	method    string
	chatID    string
	messageID string
	text      string
	markup    string
}

type keyboardClient struct {
	messageIDs map[string]int
	calls      []keyboardCall
	editError  error
}

func (c *keyboardClient) Do(req *http.Request) (*http.Response, error) {
	call := keyboardCall{
		method:    path.Base(req.URL.Path),
		chatID:    req.FormValue("chat_id"),
		messageID: req.FormValue("message_id"),
		text:      req.FormValue("text"),
		markup:    req.FormValue("reply_markup"),
	}
	c.calls = append(c.calls, call)
	if call.method == "editMessageReplyMarkup" && c.editError != nil {
		return nil, c.editError
	}
	result := "true"
	if call.method == "sendMessage" {
		c.messageIDs[call.chatID]++
		result = fmt.Sprintf(`{"message_id":%d,"chat":{"id":%s,"type":"private"}}`, c.messageIDs[call.chatID], call.chatID)
	}
	return &http.Response{Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":` + result + `}`))}, nil
}

func TestMultiselectKeepsSelectionsPerKeyboard(t *testing.T) {
	client := &keyboardClient{messageIDs: make(map[string]int)}
	b := newKeyboardTestBot(t, client)
	command := func(chatID int64) { openTestKeyboard(b, chatID) }
	click := func(chatID int64, messageID int, data string) { clickTestKeyboard(b, chatID, messageID, data) }

	command(10)
	click(10, 1, "btn_opt1")
	command(20)
	command(10)
	click(20, 1, "btn_opt3")
	click(10, 2, "btn_opt2")
	click(10, 1, "btn_opt2")
	click(10, 1, "btn_select")
	click(10, 1, "btn_select")
	click(20, 1, "btn_select")
	click(10, 2, "btn_select")

	var opened int
	var submitted []string
	var removed []string
	var expired int
	for _, call := range client.calls {
		switch {
		case call.method == "sendMessage" && call.text == "Select multiple options":
			opened++
			var markup models.InlineKeyboardMarkup
			if err := json.Unmarshal([]byte(call.markup), &markup); err != nil {
				t.Fatal(err)
			}
			if len(markup.InlineKeyboard) != 2 || len(markup.InlineKeyboard[0]) != 3 {
				t.Fatalf("unexpected new keyboard: %v", markup)
			}
			for _, button := range markup.InlineKeyboard[0] {
				if !strings.HasPrefix(button.Text, "❌") {
					t.Errorf("new keyboard shows a selected option: %q", button.Text)
				}
			}
		case call.method == "sendMessage":
			submitted = append(submitted, call.chatID+"/"+call.text)
		case call.method == "deleteMessage":
			removed = append(removed, call.chatID+"/"+call.messageID)
		case call.method == "answerCallbackQuery" && strings.Contains(call.text, "expired"):
			expired++
		}
	}
	if opened != 3 || expired != 1 {
		t.Errorf("opened %d keyboards and expired %d clicks, want 3 and 1", opened, expired)
	}
	want := []string{
		"10/Selected options: [true true false]",
		"20/Selected options: [false false true]",
		"10/Selected options: [false true false]",
	}
	if strings.Join(submitted, "\n") != strings.Join(want, "\n") {
		t.Fatalf("sent selections %v, want %v", submitted, want)
	}
	if strings.Join(removed, ",") != "10/1,20/1,10/2" {
		t.Fatalf("removed keyboards %v, want [10/1 20/1 10/2]", removed)
	}
}

func newKeyboardTestBot(t *testing.T, client *keyboardClient) *bot.Bot {
	t.Helper()
	menu := &multiselect{selections: make(map[keyboardID]selection)}
	b, err := bot.New("test", append(menu.options(), bot.WithSkipGetMe(), bot.WithHTTPClient(time.Second, client))...)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func openTestKeyboard(b *bot.Bot, chatID int64) {
	b.ProcessUpdate(context.Background(), &models.Update{Message: &models.Message{Chat: models.Chat{ID: chatID}, Text: "/select"}})
}

func clickTestKeyboard(b *bot.Bot, chatID int64, messageID int, data string) {
	b.ProcessUpdate(context.Background(), &models.Update{CallbackQuery: &models.CallbackQuery{
		ID:   "query",
		Data: data,
		Message: models.MaybeInaccessibleMessage{Message: &models.Message{
			ID:   messageID,
			Chat: models.Chat{ID: chatID},
		}},
	}})
}

func TestMultiselectKeepsSelectionWhenEditResponseIsLost(t *testing.T) {
	client := &keyboardClient{
		messageIDs: make(map[string]int),
		editError:  errors.New("response lost after applying edit"),
	}
	b := newKeyboardTestBot(t, client)
	openTestKeyboard(b, 10)
	clickTestKeyboard(b, 10, 1, "btn_opt1")
	clickTestKeyboard(b, 10, 1, "btn_select")

	var methods []string
	var messages []string
	var appliedMarkup string
	for _, call := range client.calls {
		methods = append(methods, call.method)
		switch call.method {
		case "editMessageReplyMarkup":
			appliedMarkup = call.markup
		case "sendMessage":
			messages = append(messages, call.text)
		}
	}
	const wantMethods = "sendMessage,answerCallbackQuery,editMessageReplyMarkup,sendMessage,answerCallbackQuery,sendMessage,deleteMessage"
	if strings.Join(methods, ",") != wantMethods {
		t.Errorf("Bot API calls %v, want a single callback answer before each action", methods)
	}
	if !strings.Contains(appliedMarkup, "✅ Option 1") {
		t.Errorf("keyboard edit did not select the clicked option: %s", appliedMarkup)
	}
	if len(messages) != 3 || !strings.Contains(messages[1], "recorded") || !strings.Contains(messages[1], "refresh") || messages[2] != "Selected options: [true false false]" {
		t.Errorf("sent messages %q, want the keyboard, one refresh notice and the recorded selection", messages)
	}
}
