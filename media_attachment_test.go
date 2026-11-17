package bot

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestSendPollUploadsOptionAttachments(t *testing.T) {
	client := &httpClient{
		t:       t,
		reqPath: "/bot/sendPoll",
		resp:    "{}",
		reqFields: map[string]string{
			"options": `[` +
				`{"text":"earlier option","media":{"type":"photo","media":"attach://later.jpg"}},` +
				`{"text":"later option","media":{"type":"photo","media":"attach://later.jpg"}},` +
				`{"text":"description photo","media":{"type":"photo","media":"attach://description.jpg"}},` +
				`{"text":"remote photo","media":{"type":"photo","media":"https://example.com/photo.jpg"}},` +
				`{"text":"file ID","media":{"type":"sticker","media":"sticker_file_id"}},` +
				`{"text":"link","media":{"type":"link","url":"https://example.com"}},` +
				`{"text":"no media"}]`,
			"media": `{"type":"photo","media":"attach://description.jpg"}`,
		},
		reqFiles: map[string]string{
			"later.jpg":       "option bytes",
			"description.jpg": "description bytes",
		},
	}
	b := &Bot{client: client}
	var noMedia *models.InputMediaPhoto
	_, err := b.SendPoll(context.Background(), &SendPollParams{
		ChatID:   int64(123),
		Question: "Choose an option",
		Options: []models.InputPollOption{
			{Text: "earlier option", Media: &models.InputMediaPhoto{Media: "attach://later.jpg"}},
			{Text: "later option", Media: &models.InputMediaPhoto{
				Media:           "attach://later.jpg",
				MediaAttachment: strings.NewReader("option bytes"),
			}},
			{Text: "description photo", Media: &models.InputMediaPhoto{Media: "attach://description.jpg"}},
			{Text: "remote photo", Media: &models.InputMediaPhoto{Media: "https://example.com/photo.jpg"}},
			{Text: "file ID", Media: &models.InputMediaSticker{Media: "sticker_file_id"}},
			{Text: "link", Media: &models.InputMediaLink{URL: "https://example.com"}},
			{Text: "no media", Media: noMedia},
		},
		Media: &models.InputMediaPhoto{
			Media:           "attach://description.jpg",
			MediaAttachment: strings.NewReader("description bytes"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSendPollRejectsUnresolvedOptionAttachment(t *testing.T) {
	client := &clientMock{}
	b := &Bot{client: client}
	const description = "description bytes"
	reader := strings.NewReader(description)
	_, err := b.SendPoll(context.Background(), &SendPollParams{
		ChatID:   int64(123),
		Question: "Choose an option",
		Options: []models.InputPollOption{
			{Text: "photo", Media: &models.InputMediaPhoto{Media: "attach://missing.jpg"}},
			{Text: "no media"},
		},
		Media: &models.InputMediaPhoto{Media: "attach://description.jpg", MediaAttachment: reader},
	})
	if err == nil || !strings.Contains(err.Error(), "nil attachment for attach://missing.jpg") {
		t.Fatalf("expected a missing option attachment error, got %v", err)
	}
	if client.gotReq != nil {
		t.Fatal("sent a poll with an unresolved attachment")
	}
	if reader.Len() != len(description) {
		t.Fatal("read the description attachment before checking the missing option attachment")
	}
}

func TestSendMediaGroupRejectsMissingAttachmentBeforeReadingLaterMedia(t *testing.T) {
	client := &clientMock{}
	b := &Bot{client: client}
	const photo = "photo bytes"
	reader := strings.NewReader(photo)
	_, err := b.SendMediaGroup(context.Background(), &SendMediaGroupParams{
		ChatID: int64(123),
		Media: []models.InputMedia{
			&models.InputMediaPhoto{Media: "attach://missing.jpg"},
			&models.InputMediaPhoto{Media: "attach://later.jpg", MediaAttachment: reader},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "nil attachment for attach://missing.jpg") {
		t.Fatalf("expected a missing media attachment error, got %v", err)
	}
	if client.gotReq != nil {
		t.Fatal("sent a media group with an unresolved attachment")
	}
	if reader.Len() != len(photo) {
		t.Fatal("read the later attachment before checking the missing attachment")
	}
}

func TestSendPollRejectsConflictingOptionAttachment(t *testing.T) {
	client := &clientMock{}
	b := &Bot{client: client}
	_, err := b.SendPoll(context.Background(), &SendPollParams{
		ChatID:   int64(123),
		Question: "Choose an option",
		Options: []models.InputPollOption{
			{Text: "photo", Media: &models.InputMediaPhoto{
				Media:           "attach://photo.jpg",
				MediaAttachment: strings.NewReader("option bytes"),
			}},
			{Text: "no media"},
		},
		Media: &models.InputMediaPhoto{
			Media:           "attach://photo.jpg",
			MediaAttachment: strings.NewReader("description bytes"),
		},
	})
	if err == nil || !strings.Contains(err.Error(), `duplicate form part name "photo.jpg"`) {
		t.Fatalf("expected a duplicate attachment name error, got %v", err)
	}
	if client.gotReq != nil {
		t.Fatal("sent a poll with conflicting attachments")
	}
}

func TestSendPaidMediaUploadsLivePhoto(t *testing.T) {
	client := &httpClient{
		t:       t,
		reqPath: "/bot/sendPaidMedia",
		resp:    "{}",
		reqFields: map[string]string{
			"media": `[` +
				`{"type":"live_photo","media":"attach://clip.mp4","photo":"attach://still.jpg"},` +
				`{"type":"live_photo","media":"video_id","photo":"photo_id"},` +
				`{"type":"photo","media":"attach://still.jpg"}]`,
		},
		reqFiles: map[string]string{"clip.mp4": "video bytes", "still.jpg": "photo bytes"},
	}
	live := &models.InputPaidMediaLivePhoto{
		Type:            "photo",
		Media:           "attach://clip.mp4",
		Photo:           "attach://still.jpg",
		MediaAttachment: strings.NewReader("video bytes"),
	}
	b := &Bot{client: client}
	_, err := b.SendPaidMedia(context.Background(), &SendPaidMediaParams{
		ChatID:    int64(123),
		StarCount: 1,
		Media: []models.InputPaidMedia{
			live,
			&models.InputPaidMediaLivePhoto{Media: "video_id", Photo: "photo_id"},
			&models.InputPaidMediaPhoto{Media: "attach://still.jpg", MediaAttachment: strings.NewReader("photo bytes")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if live.Type != "photo" {
		t.Fatalf("sending changed the caller's Type to %q", live.Type)
	}
}

func TestSendPaidMediaRejectsMissingLivePhotoAttachment(t *testing.T) {
	var missing *strings.Reader
	client := &clientMock{}
	b := &Bot{client: client}
	_, err := b.SendPaidMedia(context.Background(), &SendPaidMediaParams{
		ChatID:    int64(123),
		StarCount: 1,
		Media: []models.InputPaidMedia{
			&models.InputPaidMediaLivePhoto{Media: "video_id", Photo: "attach://still.jpg", PhotoAttachment: missing},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "nil PhotoAttachment for attach://still.jpg") {
		t.Fatalf("expected a missing live photo attachment error, got %v", err)
	}
	if client.gotReq != nil {
		t.Fatal("sent a paid live photo with an unresolved attachment")
	}
}

func TestWebhookReplyUploadsNestedLivePhotos(t *testing.T) {
	tests := []struct {
		method string
		params any
		field  string
		want   string
	}{
		{
			method: "sendPoll",
			params: &SendPollParams{
				ChatID:   int64(123),
				Question: "Choose an option",
				Options: []models.InputPollOption{
					{Text: "live photo", Media: &models.InputMediaLivePhoto{
						Media:           "attach://clip.mp4",
						Photo:           "attach://still.jpg",
						MediaAttachment: strings.NewReader("video bytes"),
						PhotoAttachment: strings.NewReader("photo bytes"),
					}},
					{Text: "no media"},
				},
			},
			field: "options",
			want:  `[{"text":"live photo","media":{"type":"live_photo","media":"attach://clip.mp4","photo":"attach://still.jpg"}},{"text":"no media"}]`,
		},
		{
			method: "sendPaidMedia",
			params: &SendPaidMediaParams{
				ChatID:    int64(123),
				StarCount: 1,
				Media: []models.InputPaidMedia{
					&models.InputPaidMediaLivePhoto{
						Media:           "attach://clip.mp4",
						Photo:           "attach://still.jpg",
						MediaAttachment: strings.NewReader("video bytes"),
						PhotoAttachment: strings.NewReader("photo bytes"),
					},
				},
			},
			field: "media",
			want:  `[{"type":"live_photo","media":"attach://clip.mp4","photo":"attach://still.jpg"}]`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.method, func(t *testing.T) {
			errorsHandler := &mockErrorsHandler{}
			b := newWebhookReplyTestBot(func(ctx context.Context, b *Bot, _ *models.Update) {
				if err := b.WebhookReply(ctx, tc.method, tc.params); err != nil {
					t.Fatalf("build webhook reply: %v", err)
				}
			}, errorsHandler)
			w := serveWebhookReply(t, b, &models.Update{ID: 1})
			if w.Code != http.StatusOK || len(errorsHandler.errors) != 0 {
				t.Fatalf("expected successful reply, got status %d and errors %v", w.Code, errorsHandler.errors)
			}
			mediaType, mediaParams, err := mime.ParseMediaType(w.Header().Get("Content-Type"))
			if err != nil || mediaType != "multipart/form-data" {
				t.Fatalf("expected multipart/form-data, got %q and error %v", mediaType, err)
			}
			form, err := multipart.NewReader(w.Body, mediaParams["boundary"]).ReadForm(1 << 20)
			if err != nil {
				t.Fatalf("read webhook form: %v", err)
			}
			t.Cleanup(func() {
				if err := form.RemoveAll(); err != nil {
					t.Error(err)
				}
			})
			if got := form.Value["method"]; len(got) != 1 || got[0] != tc.method {
				t.Fatalf("method = %q, want %q", got, tc.method)
			}
			if got := form.Value[tc.field]; len(got) != 1 || got[0] != tc.want {
				t.Fatalf("%s = %q, want %q", tc.field, got, tc.want)
			}
			files := map[string]string{"clip.mp4": "video bytes", "still.jpg": "photo bytes"}
			if len(form.File) != len(files) {
				t.Fatalf("got %d attachment names, want %d", len(form.File), len(files))
			}
			for name, want := range files {
				parts := form.File[name]
				if len(parts) != 1 {
					t.Fatalf("got %d parts for %q, want 1", len(parts), name)
				}
				file, err := parts[0].Open()
				if err != nil {
					t.Fatalf("open %q: %v", name, err)
				}
				data, readErr := io.ReadAll(file)
				closeErr := file.Close()
				if readErr != nil || closeErr != nil || string(data) != want {
					t.Fatalf("%q = %q, want %q (read error %v, close error %v)", name, data, want, readErr, closeErr)
				}
			}
		})
	}
}
