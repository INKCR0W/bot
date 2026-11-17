package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func TestSendPhotoUsesPublicURLAndReportsFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bot456:another-bot/sendPhoto" || r.FormValue("chat_id") != "321" {
			t.Errorf("unexpected sendPhoto request: %s, chat_id = %q", r.URL.Path, r.FormValue("chat_id"))
		}
		photo := r.FormValue("photo")
		if !strings.HasPrefix(photo, "https://raw.githubusercontent.com/go-telegram/bot/") ||
			!strings.HasSuffix(photo, "/examples/send_photo_upload/facebook.png") {
			t.Errorf("photo = %q, want the public repository fixture URL", photo)
		}
		w.WriteHeader(http.StatusBadRequest)
		if _, err := io.WriteString(w, `{"ok":false,"error_code":400,"description":"photo unavailable"}`); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()

	var output bytes.Buffer
	originalOutput := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(originalOutput)

	b, err := bot.New("456:another-bot", bot.WithServerURL(server.URL), bot.WithSkipGetMe(),
		bot.WithNotAsyncHandlers(), bot.WithDefaultHandler(handler))
	if err != nil {
		t.Fatal(err)
	}
	b.ProcessUpdate(context.Background(), &models.Update{Message: &models.Message{Chat: models.Chat{ID: 321}}})
	if !strings.Contains(output.String(), "send photo:") || !strings.Contains(output.String(), "photo unavailable") {
		t.Errorf("log = %q, want sendPhoto failure", output.String())
	}
}
