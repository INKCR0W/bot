package bot

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestStoryRequestsEncodeAreas(t *testing.T) {
	position := models.AreaPosition{XPercentage: 50, YPercentage: 50, WidthPercentage: 20, HeightPercentage: 15}
	const positionJSON = `{"x_percentage":50,"y_percentage":50,"width_percentage":20,"height_percentage":15,"rotation_angle":0,"corner_radius_percentage":0}`
	variants := []struct {
		area models.StoryAreaType
		want string
	}{
		{models.StoryAreaType{
			Type: models.StoryAreaTypeTypeLocation,
			StoryAreaTypeLocation: &models.StoryAreaTypeLocation{
				Latitude: 1.5, Longitude: 2.5, Address: &models.LocationAddress{CountryCode: "SG"},
			},
		}, `{"type":"location","latitude":1.5,"longitude":2.5,"address":{"country_code":"SG"}}`},
		{models.StoryAreaType{
			Type: models.StoryAreaTypeTypeSuggestedReaction,
			StoryAreaTypeSuggestedReaction: &models.StoryAreaTypeSuggestedReaction{
				ReactionType: models.ReactionType{
					Type:              models.ReactionTypeTypeEmoji,
					ReactionTypeEmoji: &models.ReactionTypeEmoji{Emoji: "👍"},
				},
				IsDark: true,
			},
		}, `{"type":"suggested_reaction","reaction_type":{"type":"emoji","emoji":"👍"},"is_dark":true}`},
		{models.StoryAreaType{
			Type:              models.StoryAreaTypeTypeLink,
			StoryAreaTypeLink: &models.StoryAreaTypeLink{URL: "https://example.com"},
		}, `{"type":"link","url":"https://example.com"}`},
		{models.StoryAreaType{
			Type:                 models.StoryAreaTypeTypeWeather,
			StoryAreaTypeWeather: &models.StoryAreaTypeWeather{Temperature: 25.5, Emoji: "☀️", BackgroundColor: 0x11223344},
		}, `{"type":"weather","temperature":25.5,"emoji":"☀️","background_color":287454020}`},
		{models.StoryAreaType{
			Type:                    models.StoryAreaTypeTypeUniqueGift,
			StoryAreaTypeUniqueGift: &models.StoryAreaTypeUniqueGift{Name: "gift-123"},
		}, `{"type":"unique_gift","name":"gift-123"}`},
	}
	var areas []models.StoryArea
	var areaJSON []string
	for _, variant := range variants {
		areas = append(areas, models.StoryArea{Position: position, Type: variant.area})
		areaJSON = append(areaJSON, `{"position":`+positionJSON+`,"type":`+variant.want+`}`)
	}
	for _, method := range []string{"postStory", "editStory"} {
		t.Run(method, func(t *testing.T) {
			c := &httpClient{
				t:       t,
				reqPath: "/bot/" + method,
				resp:    `{"id":123}`,
				reqFields: map[string]string{
					"areas": "[" + strings.Join(areaJSON, ",") + "]",
				},
				reqFiles: map[string]string{"story.jpg": "photo bytes"},
			}
			b := &Bot{client: c}
			content := models.InputStoryContentPhoto{Photo: "attach://story.jpg", MediaAttachment: strings.NewReader("photo bytes")}
			var story *models.Story
			var err error
			if method == "postStory" {
				story, err = b.PostStory(context.Background(), &PostStoryParams{
					BusinessConnectionID: "business-connection", ActivePeriod: 3600, Content: content, Areas: areas,
				})
			} else {
				story, err = b.EditStory(context.Background(), &EditStoryParams{
					BusinessConnectionID: "business-connection", StoryID: 123, Content: content, Areas: areas,
				})
			}
			if err != nil || story == nil || story.ID != 123 {
				t.Fatalf("got story %+v, error %v", story, err)
			}
		})
	}
}

func TestStoryAreaTypeRoundTrip(t *testing.T) {
	for _, input := range []string{`{"type":"link","url":"https://example.com"}`, `{"type":"future_type"}`} {
		var area models.StoryAreaType
		if err := json.Unmarshal([]byte(input), &area); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(area)
		if err != nil || string(encoded) != input {
			t.Errorf("%s encoded as %s, %v", input, encoded, err)
		}
	}
	for _, area := range []models.StoryAreaType{{}, {Type: models.StoryAreaTypeTypeLink}} {
		if _, err := json.Marshal(area); err == nil {
			t.Errorf("expected an encoding error for incomplete story area %+v", area)
		}
	}
}
