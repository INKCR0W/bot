package bot

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestMessageMethodsKeepOptionalOverrides(t *testing.T) {
	zero, timestamp := 0, 17
	empty, caption := "", "A \"quoted\" caption\n<example>"
	for _, tc := range []struct {
		name      string
		timestamp *int
		caption   *string
	}{
		{name: "keep original"},
		{name: "explicit zero and empty", timestamp: &zero, caption: &empty},
		{name: "explicit nonzero and nonempty", timestamp: &timestamp, caption: &caption},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &httpClient{
				t:       t,
				resp:    `{"message_id":11}`,
				reqPath: "/bot/forwardMessage",
				reqFields: map[string]string{
					"chat_id":      "123",
					"from_chat_id": "456",
					"message_id":   "7",
				},
			}
			if tc.timestamp == nil {
				client.absent = []string{"video_start_timestamp"}
			} else {
				client.reqFields["video_start_timestamp"] = strconv.Itoa(*tc.timestamp)
			}
			b := &Bot{client: client}
			_, err := b.ForwardMessage(context.Background(), &ForwardMessageParams{
				ChatID: 123, FromChatID: 456, MessageID: 7, VideoStartTimestamp: tc.timestamp,
			})
			if err != nil {
				t.Fatal(err)
			}

			client.reqPath = "/bot/copyMessage"
			if tc.caption == nil {
				client.absent = append(client.absent, "caption")
			} else {
				client.reqFields["caption"] = *tc.caption
				client.present = []string{"caption"}
			}
			client.reqFields["reply_parameters"] = `{"message_id":5}`
			client.reqFields["reply_markup"] = `{"inline_keyboard":[[{"text":"Open","url":"https://example.org"}]]}`
			_, err = b.CopyMessage(context.Background(), &CopyMessageParams{
				ChatID: 123, FromChatID: 456, MessageID: 7, VideoStartTimestamp: tc.timestamp, Caption: tc.caption,
				ReplyParameters: &models.ReplyParameters{MessageID: 5},
				ReplyMarkup: models.InlineKeyboardMarkup{
					InlineKeyboard: [][]models.InlineKeyboardButton{{{Text: "Open", URL: "https://example.org"}}},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAnswerInlineQueryKeepsOptionalCacheTime(t *testing.T) {
	zero, cacheTime := 0, 60
	for _, tc := range []struct {
		name      string
		cacheTime *int
	}{
		{name: "default"},
		{name: "explicit zero", cacheTime: &zero},
		{name: "explicit nonzero", cacheTime: &cacheTime},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &httpClient{
				t: t, resp: `true`, reqPath: "/bot/answerInlineQuery",
				reqFields: map[string]string{"inline_query_id": "42", "results": "[]"},
			}
			if tc.cacheTime == nil {
				client.absent = []string{"cache_time"}
			} else {
				client.reqFields["cache_time"] = strconv.Itoa(*tc.cacheTime)
			}
			b := &Bot{client: client}
			ok, err := b.AnswerInlineQuery(context.Background(), &AnswerInlineQueryParams{
				InlineQueryID: "42", Results: []models.InlineQueryResult{}, CacheTime: tc.cacheTime,
			})
			if err != nil || !ok {
				t.Fatalf("AnswerInlineQuery = %v, %v", ok, err)
			}

			errorsHandler := &mockErrorsHandler{}
			replyBot := newWebhookReplyTestBot(func(ctx context.Context, b *Bot, update *models.Update) {
				err := b.WebhookReply(ctx, "answerInlineQuery", &models.AnswerInlineQuery{
					InlineQueryID: update.InlineQuery.ID, Results: []models.InlineQueryResult{}, CacheTime: tc.cacheTime,
				})
				if err != nil {
					t.Errorf("WebhookReply: %v", err)
				}
			}, errorsHandler)
			w := serveWebhookReply(t, replyBot, &models.Update{ID: 1, InlineQuery: &models.InlineQuery{ID: "42"}})
			if w.Code != http.StatusOK || len(errorsHandler.errors) != 0 {
				t.Fatalf("webhook status %d, errors %v", w.Code, errorsHandler.errors)
			}
			form := readWebhookReplyForm(t, w)
			want, exists := client.reqFields["cache_time"]
			got := form["cache_time"]
			if (!exists && len(got) != 0) || (exists && (len(got) != 1 || got[0] != want)) {
				t.Fatalf("webhook cache_time = %q, want %q (present: %v)", got, want, exists)
			}
		})
	}
}

func TestEditForumTopicKeepsOptionalIconChange(t *testing.T) {
	empty, icon := "", "5368324170671202286"
	for _, tc := range []struct {
		name string
		icon *string
	}{
		{name: "keep"},
		{name: "remove", icon: &empty},
		{name: "replace", icon: &icon},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &httpClient{
				t: t, resp: `true`, reqPath: "/bot/editForumTopic",
				reqFields: map[string]string{"chat_id": "-100123", "message_thread_id": "7"},
			}
			if tc.icon == nil {
				client.absent = []string{"icon_custom_emoji_id"}
			} else {
				client.reqFields["icon_custom_emoji_id"] = *tc.icon
				client.present = []string{"icon_custom_emoji_id"}
			}
			b := &Bot{client: client}
			ok, err := b.EditForumTopic(context.Background(), &EditForumTopicParams{
				ChatID: -100123, MessageThreadID: 7, IconCustomEmojiID: tc.icon,
			})
			if err != nil || !ok {
				t.Fatalf("EditForumTopic = %v, %v", ok, err)
			}
		})
	}
}

func TestChatMenuMethodsUseDefaultWithoutBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
		}
		if len(body) != 0 || req.Header.Get("Content-Type") != "" {
			t.Errorf("%s sent a body with content type %q: %q", req.URL.Path, req.Header.Get("Content-Type"), body)
		}
		var response string
		switch req.URL.Path {
		case "/bot/getChatMenuButton":
			response = `{"ok":true,"result":{"type":"default"}}`
		case "/bot/setChatMenuButton":
			response = `{"ok":true,"result":true}`
		default:
			t.Errorf("unexpected request path %q", req.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if _, err = io.WriteString(w, response); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	b := &Bot{url: server.URL, client: server.Client()}
	button, err := b.GetChatMenuButton(context.Background(), &GetChatMenuButtonParams{})
	if err != nil || button.Type != models.MenuButtonTypeDefault {
		t.Fatalf("GetChatMenuButton = %+v, %v", button, err)
	}
	ok, err := b.SetChatMenuButton(context.Background(), &SetChatMenuButtonParams{})
	if err != nil || !ok {
		t.Fatalf("SetChatMenuButton = %v, %v", ok, err)
	}
}

func TestSetChatMenuButtonOmitsOnlyUnsetButton(t *testing.T) {
	var nilButton *models.MenuButtonDefault
	for _, tc := range []struct {
		name   string
		button models.InputMenuButton
		want   string
	}{
		{name: "unset"},
		{name: "typed nil", button: nilButton},
		{name: "explicit default", button: models.MenuButtonDefault{Type: models.MenuButtonTypeDefault}, want: `{"type":"default"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &httpClient{
				t: t, resp: `true`, reqPath: "/bot/setChatMenuButton",
				reqFields: map[string]string{"chat_id": "123"},
			}
			if tc.want == "" {
				client.absent = []string{"menu_button"}
			} else {
				client.reqFields["menu_button"] = tc.want
			}
			b := &Bot{client: client}
			ok, err := b.SetChatMenuButton(context.Background(), &SetChatMenuButtonParams{ChatID: 123, MenuButton: tc.button})
			if err != nil || !ok {
				t.Fatalf("SetChatMenuButton = %v, %v", ok, err)
			}
		})
	}
}

func TestSetStickerKeywordsKeepsOptionalList(t *testing.T) {
	for _, tc := range []struct {
		name     string
		keywords []string
		want     string
	}{
		{name: "omitted"},
		{name: "explicit empty list", keywords: []string{}, want: "[]"},
		{name: "explicit keywords", keywords: []string{"waves", "hello"}, want: `["waves","hello"]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &httpClient{
				t: t, resp: `true`, reqPath: "/bot/setStickerKeywords",
				reqFields: map[string]string{"sticker": "sticker-id"},
			}
			if tc.want == "" {
				client.absent = []string{"keywords"}
			} else {
				client.reqFields["keywords"] = tc.want
			}
			b := &Bot{client: client}
			ok, err := b.SetStickerKeywords(context.Background(), &SetStickerKeywordsParams{Sticker: "sticker-id", Keywords: tc.keywords})
			if err != nil || !ok {
				t.Fatalf("SetStickerKeywords = %v, %v", ok, err)
			}
		})
	}
}
