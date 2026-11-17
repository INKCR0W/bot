package bot

import (
	"context"
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestBot_AnswerInlineQueryWithRichMessage(t *testing.T) {
	client := &httpClient{
		t:       t,
		resp:    `true`,
		reqPath: "/bot123:token/answerInlineQuery",
		reqFields: map[string]string{
			"inline_query_id": "query-id",
			"results":         `[{"type":"article","id":"article-id","title":"Greeting","input_message_content":{"rich_message":{"markdown":"**Hello**"}}}]`,
		},
	}
	b, err := New("123:token", WithHTTPClient(defaultPollTimeout, client), WithSkipGetMe())
	if err != nil {
		t.Fatal(err)
	}
	ok, err := b.AnswerInlineQuery(context.Background(), &AnswerInlineQueryParams{
		InlineQueryID: "query-id",
		Results: []models.InlineQueryResult{&models.InlineQueryResultArticle{
			ID:    "article-id",
			Title: "Greeting",
			InputMessageContent: models.InputRichMessageContent{
				RichMessage: models.InputRichMessage{Markdown: "**Hello**"},
			},
		}},
	})
	if err != nil || !ok {
		t.Fatalf("AnswerInlineQuery() = %v, %v", ok, err)
	}
}
