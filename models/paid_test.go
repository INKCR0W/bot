package models

import "testing"

func TestPaidMedia_UnmarshalJSON_Preview(t *testing.T) {
	src := `{"type":"preview","width":42,"height":43,"duration":44}`

	p := &PaidMedia{}
	err := p.UnmarshalJSON([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if p.Type != PaidMediaTypePreview {
		t.Fatal("wrong type")
	}

	if p.Preview == nil {
		t.Fatal("Preview is nil")
	}

	if p.Preview.Width != 42 {
		t.Fatal("wrong width")
	}

	if p.Preview.Height != 43 {
		t.Fatal("wrong height")
	}

	if p.Preview.Duration != 44 {
		t.Fatal("wrong duration")
	}
}

func TestPaidMedia_UnmarshalJSON_Photo(t *testing.T) {
	src := `{"type":"photo","photo":[{"width":42,"height":43}]}`

	p := &PaidMedia{}
	err := p.UnmarshalJSON([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if p.Type != PaidMediaTypePhoto {
		t.Fatal("wrong type")
	}

	if p.Photo == nil {
		t.Fatal("Photo is nil")
	}

	if len(p.Photo.Photo) != 1 {
		t.Fatal("wrong photo length")
	}

	if p.Photo.Photo[0].Width != 42 {
		t.Fatal("wrong width")
	}
	if p.Photo.Photo[0].Height != 43 {
		t.Fatal("wrong height")
	}
}

func TestPaidMedia_UnmarshalJSON_Video(t *testing.T) {
	src := `{"type":"video","video":{"width":42,"height":43,"duration":44}}`

	p := &PaidMedia{}
	err := p.UnmarshalJSON([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if p.Type != PaidMediaTypeVideo {
		t.Fatal("wrong type")
	}

	if p.Video == nil {
		t.Fatal("Video is nil")
	}

	if p.Video.Video.Width != 42 {
		t.Fatal("wrong width")
	}
	if p.Video.Video.Height != 43 {
		t.Fatal("wrong height")
	}
	if p.Video.Video.Duration != 44 {
		t.Fatal("wrong duration")
	}
}

func TestPaidMedia_RoundTrip(t *testing.T) {
	for _, c := range []struct {
		tag  string
		data string
	}{
		{"preview", `{"type":"preview","width":42,"height":43,"duration":44}`},
		{"photo", `{"type":"photo","photo":[{"file_id":"photo-42","width":42,"height":43}]}`},
		{"video", `{"type":"video","video":{"file_id":"video-42","width":42,"height":43,"duration":44}}`},
		{"live_photo", `{"type":"live_photo","live_photo":{"file_id":"live-42","width":42,"height":43,"duration":44}}`},
	} {
		t.Run(c.tag, func(t *testing.T) {
			value := assertTaggedRoundTrip[PaidMedia](t, c.data, c.tag)
			if c.tag == "live_photo" && (value.LivePhoto == nil || value.LivePhoto.LivePhoto.FileID != "live-42") {
				t.Fatalf("live photo payload was not decoded: %+v", value.LivePhoto)
			}
		})
	}
}

func TestPaidMedia_LegacyArchive(t *testing.T) {
	const legacy = `{"Type":"photo","Preview":null,"Photo":{"Type":"","photo":[{"file_id":"photo-42","width":42}]},"Video":null}`
	value := assertTaggedRoundTrip[PaidMedia](t, legacy, "photo")
	if value.Photo == nil || value.Photo.Type != PaidMediaTypePhoto || len(value.Photo.Photo) != 1 || value.Photo.Photo[0].FileID != "photo-42" {
		t.Fatalf("legacy archive lost photo information: %+v", value.Photo)
	}
}
