package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-telegram/bot"
)

type echoClient struct {
	calls int32
}

func (c *echoClient) Do(*http.Request) (*http.Response, error) {
	atomic.AddInt32(&c.calls, 1)
	return nil, errors.New("unexpected Bot API request")
}

type startContext struct {
	context.Context
	started chan struct{}
	once    sync.Once
}

func (c *startContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.started) })
	return c.Context.Done()
}

func TestWebhookEchoIgnoresEditedMessage(t *testing.T) {
	client := &echoClient{}
	b, err := bot.New("test", bot.WithSkipGetMe(), bot.WithDefaultHandler(handler), bot.WithHTTPClient(time.Second, client))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		b.StartWebhook(&startContext{Context: ctx, started: started})
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("webhook did not start")
	}

	const payload = `{"update_id":1,"edited_message":{"message_id":7,"date":123,"chat":{"id":42,"type":"private"},"text":"edited"}}`
	response := httptest.NewRecorder()
	b.WebhookHandler()(response, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload)))
	if response.Code != http.StatusOK {
		t.Errorf("webhook status %d, want 200", response.Code)
	}
	cancel()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("webhook did not finish processing the update")
	}
	if calls := atomic.LoadInt32(&client.calls); calls != 0 {
		t.Fatalf("made %d Bot API requests for an edited message", calls)
	}
}
