package bot

import (
	"context"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestStickerSetSingleSticker(t *testing.T) {
	tests := []struct {
		method string
		send   func(*Bot, models.InputSticker) (bool, error)
	}{
		{"addStickerToSet", func(b *Bot, sticker models.InputSticker) (bool, error) {
			return b.AddStickerToSet(context.Background(), &AddStickerToSetParams{
				UserID: 123, Name: "set_by_bot", Sticker: sticker,
			})
		}},
		{"replaceStickerInSet", func(b *Bot, sticker models.InputSticker) (bool, error) {
			return b.ReplaceStickerInSet(context.Background(), &ReplaceStickerInSetParams{
				UserID: 123, Name: "set_by_bot", OldSticker: "old-file-id", Sticker: sticker,
			})
		}},
	}
	for _, test := range tests {
		t.Run(test.method, func(t *testing.T) {
			for _, reference := range []string{"attach://sticker.webp", "telegram-file-id"} {
				t.Run(reference, func(t *testing.T) {
					sticker := models.InputSticker{Sticker: reference, Format: "static", EmojiList: []string{"🙂"}}
					c := &httpClient{
						t:       t,
						reqPath: "/bot/" + test.method,
						resp:    `true`,
						reqFields: map[string]string{
							"sticker": `{"sticker":"` + reference + `","format":"static","emoji_list":["🙂"]}`,
						},
					}
					if strings.HasPrefix(reference, "attach://") {
						sticker.StickerAttachment = strings.NewReader("sticker bytes")
						c.reqFiles = map[string]string{"sticker.webp": "sticker bytes"}
					}
					ok, err := test.send(&Bot{client: c}, sticker)
					if err != nil || !ok {
						t.Fatalf("got result %v, error %v", ok, err)
					}
				})
			}
		})
	}
}

func TestAddStickerToSetRejectsInvalidAttachment(t *testing.T) {
	for _, test := range []struct {
		name    string
		sticker models.InputSticker
		want    string
	}{
		{"missing", models.InputSticker{Sticker: "attach://sticker.webp", StickerAttachment: (*strings.Reader)(nil)}, "nil StickerAttachment for attach://sticker.webp"},
		{"empty name", models.InputSticker{Sticker: "attach://", StickerAttachment: strings.NewReader("bytes")}, "empty name for StickerAttachment"},
		{"conflict", models.InputSticker{Sticker: "attach://sticker", StickerAttachment: strings.NewReader("bytes")}, `duplicate form part name "sticker"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := &clientMock{}
			b := &Bot{client: c}
			_, err := b.AddStickerToSet(context.Background(), &AddStickerToSetParams{Sticker: test.sticker})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want error containing %q, got %v", test.want, err)
			}
			if c.gotReq != nil {
				t.Fatal("sent a request with an invalid sticker attachment")
			}
		})
	}
}
