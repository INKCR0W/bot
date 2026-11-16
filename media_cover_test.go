package bot

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestSendMediaGroupUploadsVideoCover(t *testing.T) {
	client := &httpClient{
		t:       t,
		reqPath: "/bot/sendMediaGroup",
		resp:    "[]",
		reqFields: map[string]string{
			"media": `[{"type":"video","media":"attach://video.mp4","thumbnail":"attach://thumb.jpg","cover":"attach://cover.jpg"},{"type":"video","media":"video_id","cover":"cover_id"},{"type":"video","media":"video_id","cover":"https://example.com/cover.jpg"}]`,
		},
		reqFiles: map[string]string{
			"video.mp4": "video bytes",
			"thumb.jpg": "thumbnail bytes",
			"cover.jpg": "cover bytes",
		},
	}
	b := &Bot{client: client}
	_, err := b.SendMediaGroup(context.Background(), &SendMediaGroupParams{
		ChatID: int64(123),
		Media: []models.InputMedia{
			&models.InputMediaVideo{
				Media:           "attach://video.mp4",
				Thumbnail:       &models.InputFileUpload{Filename: "thumb.jpg", Data: strings.NewReader("thumbnail bytes")},
				Cover:           "attach://cover.jpg",
				MediaAttachment: strings.NewReader("video bytes"),
				CoverAttachment: strings.NewReader("cover bytes"),
			},
			&models.InputMediaVideo{Media: "video_id", Cover: "cover_id"},
			&models.InputMediaVideo{Media: "video_id", Cover: "https://example.com/cover.jpg"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSendPaidMediaUploadsVideoCover(t *testing.T) {
	client := &httpClient{
		t:       t,
		reqPath: "/bot/sendPaidMedia",
		resp:    "{}",
		reqFields: map[string]string{
			"media": `[{"type":"video","media":"attach://paid.mp4","thumbnail":"attach://paid_thumb.jpg","cover":"attach://paid_cover.jpg"}]`,
		},
		reqFiles: map[string]string{
			"paid.mp4":       "paid video bytes",
			"paid_thumb.jpg": "paid thumbnail bytes",
			"paid_cover.jpg": "paid cover bytes",
		},
	}
	b := &Bot{client: client}
	_, err := b.SendPaidMedia(context.Background(), &SendPaidMediaParams{
		ChatID:    int64(123),
		StarCount: 1,
		Media: []models.InputPaidMedia{
			&models.InputPaidMediaVideo{
				Media:           "attach://paid.mp4",
				Thumbnail:       &models.InputFileUpload{Filename: "paid_thumb.jpg", Data: strings.NewReader("paid thumbnail bytes")},
				Cover:           "attach://paid_cover.jpg",
				MediaAttachment: strings.NewReader("paid video bytes"),
				CoverAttachment: strings.NewReader("paid cover bytes"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSendMediaGroupCoverUsesLaterPart(t *testing.T) {
	client := &httpClient{
		t:       t,
		reqPath: "/bot/sendMediaGroup",
		resp:    "[]",
		reqFields: map[string]string{
			"media": `[{"type":"video","media":"video_id","cover":"attach://photo.jpg"},{"type":"photo","media":"attach://photo.jpg"}]`,
		},
		reqFiles: map[string]string{"photo.jpg": "shared photo bytes"},
	}
	b := &Bot{client: client}
	_, err := b.SendMediaGroup(context.Background(), &SendMediaGroupParams{
		ChatID: int64(123),
		Media: []models.InputMedia{
			&models.InputMediaVideo{Media: "video_id", Cover: "attach://photo.jpg"},
			&models.InputMediaPhoto{Media: "attach://photo.jpg", MediaAttachment: strings.NewReader("shared photo bytes")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSendPaidMediaRejectsMissingCoverPart(t *testing.T) {
	b := &Bot{client: &httpClient{t: t, resp: "{}"}}
	var missingCover *strings.Reader
	_, err := b.SendPaidMedia(context.Background(), &SendPaidMediaParams{
		ChatID:    int64(123),
		StarCount: 1,
		Media: []models.InputPaidMedia{
			&models.InputPaidMediaVideo{Media: "video_id", Cover: "attach://missing.jpg", CoverAttachment: missingCover},
		},
	})
	if err == nil || !strings.Contains(err.Error(), `missing video cover part for "attach://missing.jpg"`) {
		t.Fatalf("expected a missing cover error, got %v", err)
	}
}

type coverReaderFunc func([]byte) (int, error)

func (r coverReaderFunc) Read(p []byte) (int, error) {
	return r(p)
}

type coverReaderArray [1]io.Reader

func (r coverReaderArray) Read(p []byte) (int, error) {
	return r[0].Read(p)
}

func TestSendMediaGroupRejectsDuplicateCoverWithIncomparableReader(t *testing.T) {
	for _, tc := range []struct {
		name string
		wrap func(io.Reader) io.Reader
	}{
		{"embedded interface", func(r io.Reader) io.Reader { return struct{ io.Reader }{r} }},
		{"interface array", func(r io.Reader) io.Reader { return coverReaderArray{r} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := tc.wrap(coverReaderFunc(strings.NewReader("cover bytes").Read))
			b := &Bot{client: &httpClient{t: t, resp: "[]"}}
			_, err := b.SendMediaGroup(context.Background(), &SendMediaGroupParams{
				ChatID: int64(123),
				Media: []models.InputMedia{
					&models.InputMediaVideo{Media: "first_id", Cover: "attach://cover.jpg", CoverAttachment: reader},
					&models.InputMediaVideo{Media: "second_id", Cover: "attach://cover.jpg", CoverAttachment: reader},
				},
			})
			if err == nil || !strings.Contains(err.Error(), `duplicate form part name "cover.jpg"`) {
				t.Fatalf("expected a duplicate cover error, got %v", err)
			}
		})
	}
}

func TestSendMediaGroupSharesCoverWithSameReader(t *testing.T) {
	reader := struct{ io.Reader }{strings.NewReader("cover bytes")}
	client := &httpClient{
		t:       t,
		reqPath: "/bot/sendMediaGroup",
		resp:    "[]",
		reqFields: map[string]string{
			"media": `[{"type":"video","media":"first_id","cover":"attach://cover.jpg"},{"type":"video","media":"second_id","cover":"attach://cover.jpg"}]`,
		},
		reqFiles: map[string]string{"cover.jpg": "cover bytes"},
	}
	b := &Bot{client: client}
	_, err := b.SendMediaGroup(context.Background(), &SendMediaGroupParams{
		ChatID: int64(123),
		Media: []models.InputMedia{
			&models.InputMediaVideo{Media: "first_id", Cover: "attach://cover.jpg", CoverAttachment: reader},
			&models.InputMediaVideo{Media: "second_id", Cover: "attach://cover.jpg", CoverAttachment: reader},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}
