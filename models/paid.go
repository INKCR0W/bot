package models

import (
	"encoding/json"
	"io"
)

type InputPaidMedia interface {
	inputPaidMediaTag()

	MarshalInputMedia() ([]byte, error)
	Attachment() io.Reader
	GetMedia() string
}

// InputPaidMediaPhoto https://core.telegram.org/bots/api#inputpaidmediaphoto
type InputPaidMediaPhoto struct {
	Media string `json:"media"`

	MediaAttachment io.Reader `json:"-"`
}

func (m *InputPaidMediaPhoto) inputPaidMediaTag() {}

func (m *InputPaidMediaPhoto) Attachment() io.Reader {
	return m.MediaAttachment
}

func (m *InputPaidMediaPhoto) GetMedia() string {
	return m.Media
}

func (m *InputPaidMediaPhoto) MarshalInputMedia() ([]byte, error) {
	ret := struct {
		Type string `json:"type"`
		*InputPaidMediaPhoto
	}{
		Type:                "photo",
		InputPaidMediaPhoto: m,
	}

	return json.Marshal(&ret)
}

// InputPaidMediaVideo https://core.telegram.org/bots/api#inputpaidmediavideo
type InputPaidMediaVideo struct {
	Media             string    `json:"media"`
	Thumbnail         InputFile `json:"thumbnail,omitempty"`
	Cover             string    `json:"cover,omitempty"`
	StartTimestamp    int       `json:"start_timestamp,omitempty"`
	Width             int       `json:"width,omitempty"`
	Height            int       `json:"height,omitempty"`
	Duration          int       `json:"duration,omitempty"`
	SupportsStreaming bool      `json:"supports_streaming,omitempty"`

	MediaAttachment io.Reader `json:"-"`
	// CoverAttachment supplies the named part when Cover uses attach://.
	// Leave it nil to reference a part supplied elsewhere in the request.
	CoverAttachment io.Reader `json:"-"`
}

func (m *InputPaidMediaVideo) inputPaidMediaTag() {}

func (m *InputPaidMediaVideo) Attachment() io.Reader {
	return m.MediaAttachment
}

func (m *InputPaidMediaVideo) GetMedia() string {
	return m.Media
}

func (m *InputPaidMediaVideo) GetCover() (string, io.Reader) {
	return m.Cover, m.CoverAttachment
}

func (m *InputPaidMediaVideo) GetThumbnail() InputFile {
	return m.Thumbnail
}

func (m *InputPaidMediaVideo) MarshalInputMedia() ([]byte, error) {
	normalized := *m
	normalized.Thumbnail = normalizeInputFile(normalized.Thumbnail)

	ret := struct {
		Type string `json:"type"`
		*InputPaidMediaVideo
	}{
		Type:                "video",
		InputPaidMediaVideo: &normalized,
	}

	return json.Marshal(&ret)
}

type PaidMediaType string

const (
	PaidMediaTypePreview   PaidMediaType = "preview"
	PaidMediaTypePhoto     PaidMediaType = "photo"
	PaidMediaTypeVideo     PaidMediaType = "video"
	PaidMediaTypeLivePhoto PaidMediaType = "live_photo"
)

// PaidMedia https://core.telegram.org/bots/api#paidmedia
type PaidMedia struct {
	Type PaidMediaType

	Preview   *PaidMediaPreview
	Photo     *PaidMediaPhoto
	Video     *PaidMediaVideo
	LivePhoto *PaidMediaLivePhoto
}

func (p PaidMedia) MarshalJSON() ([]byte, error) {
	switch p.Type {
	case PaidMediaTypePreview:
		return marshalVariant("PaidMedia", p.Type, p.Preview, func(v *PaidMediaPreview) { v.Type = p.Type })
	case PaidMediaTypePhoto:
		return marshalVariant("PaidMedia", p.Type, p.Photo, func(v *PaidMediaPhoto) { v.Type = p.Type })
	case PaidMediaTypeVideo:
		return marshalVariant("PaidMedia", p.Type, p.Video, func(v *PaidMediaVideo) { v.Type = p.Type })
	case PaidMediaTypeLivePhoto:
		return marshalVariant("PaidMedia", p.Type, p.LivePhoto, func(v *PaidMediaLivePhoto) { v.Type = string(p.Type) })
	}

	return marshalUnknownVariant("PaidMedia", "type", p.Type)
}

func (p *PaidMedia) UnmarshalJSON(data []byte) error {
	v := struct {
		Type       PaidMediaType `json:"type"`
		LegacyType PaidMediaType `json:"Type"`
	}{}
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}
	legacy := v.Type == "" && v.LegacyType != ""
	if legacy {
		v.Type = v.LegacyType
	}

	if v.Type == "" {
		return missingDiscriminator("PaidMedia")
	}

	decoded := PaidMedia{Type: v.Type}
	var variant any
	var archivedField string
	switch v.Type {
	case PaidMediaTypePreview:
		decoded.Preview = &PaidMediaPreview{}
		variant, archivedField = decoded.Preview, "Preview"
	case PaidMediaTypePhoto:
		decoded.Photo = &PaidMediaPhoto{}
		variant, archivedField = decoded.Photo, "Photo"
	case PaidMediaTypeVideo:
		decoded.Video = &PaidMediaVideo{}
		variant, archivedField = decoded.Video, "Video"
	case PaidMediaTypeLivePhoto:
		decoded.LivePhoto = &PaidMediaLivePhoto{}
		variant = decoded.LivePhoto
	}

	if legacy && archivedField != "" {
		archived, err := archivedVariantField(data, archivedField, string(v.Type))
		if err != nil {
			return err
		}
		if archived != nil {
			data = archived
		}
	}
	if variant != nil {
		if err := json.Unmarshal(data, variant); err != nil {
			return err
		}
	}

	*p = decoded
	return nil
}

// PaidMediaPreview https://core.telegram.org/bots/api#paidmediapreview
type PaidMediaPreview struct {
	Type PaidMediaType `json:"type"`

	Width    int `json:"width,omitempty"`
	Height   int `json:"height,omitempty"`
	Duration int `json:"duration,omitempty"`
}

// PaidMediaPhoto https://core.telegram.org/bots/api#paidmediaphoto
type PaidMediaPhoto struct {
	Type PaidMediaType `json:"type"`

	Photo []PhotoSize `json:"photo"`
}

// PaidMediaVideo https://core.telegram.org/bots/api#paidmediavideo
type PaidMediaVideo struct {
	Type PaidMediaType `json:"type"`

	Video Video `json:"video"`
}

// PaidMediaInfo https://core.telegram.org/bots/api#paidmediainfo
type PaidMediaInfo struct {
	StarCount int         `json:"star_count"`
	PaidMedia []PaidMedia `json:"paid_media"`
}

// PaidMediaPurchased https://core.telegram.org/bots/api#paidmediapurchased
type PaidMediaPurchased struct {
	From             User   `json:"from"`
	PaidMediaPayload string `json:"paid_media_payload"`
}

// PaidMessagePriceChanged https://core.telegram.org/bots/api#paidmessagepricechanged
type PaidMessagePriceChanged struct {
	PaidMessageStarCount int `json:"paid_message_star_count"`
}

// DirectMessagePriceChanged https://core.telegram.org/bots/api#directmessagepricechanged
type DirectMessagePriceChanged struct {
	AreDirectMessagesEnabled bool `json:"are_direct_messages_enabled"`
	DirectMessageStarCount   int  `json:"direct_message_star_count,omitempty"`
}
