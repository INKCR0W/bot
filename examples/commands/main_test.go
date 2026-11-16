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

type commandClient struct {
	texts []string
}

func (c *commandClient) Do(req *http.Request) (*http.Response, error) {
	c.texts = append(c.texts, req.FormValue("text"))
	return &http.Response{Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":1}}`))}, nil
}

func TestCommandsIgnoreUpdatesWithoutRequiredMessageFields(t *testing.T) {
	client := &commandClient{}
	b, err := bot.New("test", bot.WithSkipGetMe(), bot.WithNotAsyncHandlers(),
		bot.WithDefaultHandler(defaultHandler),
		bot.WithMessageTextHandler("/hello", bot.MatchTypeExact, helloHandler),
		bot.WithHTTPClient(time.Second, client),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	b.ProcessUpdate(ctx, &models.Update{EditedMessage: &models.Message{Text: "edited"}})
	b.ProcessUpdate(ctx, &models.Update{Message: &models.Message{Text: "/hello"}})
	if len(client.texts) != 0 {
		t.Fatalf("sent %v for updates without required message fields", client.texts)
	}

	b.ProcessUpdate(ctx, &models.Update{Message: &models.Message{Text: "/hello", From: &models.User{FirstName: "Ada"}}})
	if len(client.texts) != 1 || client.texts[0] != "Hello, *Ada*" {
		t.Fatalf("sent %v for /hello, want greeting", client.texts)
	}
}
