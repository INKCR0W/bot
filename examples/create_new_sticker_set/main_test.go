package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type stickerClient func(*http.Request) (*http.Response, error)

func (c stickerClient) Do(r *http.Request) (*http.Response, error) { return c(r) }

func TestCreateNewStickerSetUsesHumanSenderAsOwner(t *testing.T) {
	var stickerSetName string
	var calls []string
	client := stickerClient(func(r *http.Request) (*http.Response, error) {
		defer r.Body.Close()
		calls = append(calls, r.URL.Path)
		var response string
		switch r.URL.Path {
		case "/bot123:test/createNewStickerSet":
			if got := r.FormValue("user_id"); got != "456" {
				t.Errorf("sticker set user_id = %q, want sender ID 456", got)
			}
			stickerSetName = r.FormValue("name")
			if stickerSetName == "" {
				t.Error("missing sticker set name")
			}
			response = `{"ok":true,"result":true}`
		case "/bot123:test/getStickerSet":
			if got := r.FormValue("name"); got != stickerSetName {
				t.Errorf("getStickerSet name = %q, want %q", got, stickerSetName)
			}
			response = `{"ok":true,"result":{"stickers":[{"file_id":"test-sticker"}]}}`
		case "/bot123:test/sendSticker":
			if got := r.FormValue("chat_id"); got != "-321" {
				t.Errorf("sendSticker chat_id = %q, want group ID -321", got)
			}
			if got := r.FormValue("sticker"); got != "test-sticker" {
				t.Errorf("sendSticker sticker = %q, want file from the new sticker set", got)
			}
			response = `{"ok":true,"result":{"message_id":1}}`
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(response))}, nil
	})
	b, err := bot.New("123:test", bot.WithHTTPClient(time.Second, client), bot.WithSkipGetMe(),
		bot.WithNotAsyncHandlers(), bot.WithDefaultHandler(handler))
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	chat := models.Chat{ID: -321, Type: models.ChatTypeSupergroup}
	b.ProcessUpdate(ctx, &models.Update{Message: &models.Message{
		Chat: chat, From: &models.User{ID: 456},
	}})
	const wantCalls = "/bot123:test/createNewStickerSet,/bot123:test/getStickerSet,/bot123:test/sendSticker"
	if got := strings.Join(calls, ","); got != wantCalls {
		t.Fatalf("API calls = %q, want %q", got, wantCalls)
	}

	calls = nil
	b.ProcessUpdate(ctx, &models.Update{Message: &models.Message{Chat: chat}})
	b.ProcessUpdate(ctx, &models.Update{Message: &models.Message{
		Chat: chat, SenderChat: &chat, From: &models.User{ID: 1, IsBot: true},
	}})
	if len(calls) != 0 {
		t.Errorf("API calls without a real sender = %v, want none", calls)
	}
}
