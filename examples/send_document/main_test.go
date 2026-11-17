package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func TestSendDocumentUploadsEmbeddedFixture(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.URL.Path != "/bot123:test/sendDocument" || r.FormValue("chat_id") != "-321" {
			t.Errorf("unexpected sendDocument request: %s, chat_id = %q", r.URL.Path, r.FormValue("chat_id"))
		}
		file, header, err := r.FormFile("document")
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer file.Close()
		content, err := io.ReadAll(file)
		if err != nil || string(content) != "A document body" || header.Filename != "demo.txt" {
			t.Errorf("document upload = %q, %q, %v", header.Filename, content, err)
		}
		if _, err := io.WriteString(w, `{"ok":true,"result":{"message_id":1}}`); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()

	b, err := bot.New("123:test", bot.WithServerURL(server.URL), bot.WithSkipGetMe(),
		bot.WithNotAsyncHandlers(), bot.WithDefaultHandler(handler))
	if err != nil {
		t.Fatal(err)
	}
	b.ProcessUpdate(context.Background(), &models.Update{Message: &models.Message{Chat: models.Chat{ID: -321}}})
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("sendDocument calls = %d, want 1", got)
	}
}
