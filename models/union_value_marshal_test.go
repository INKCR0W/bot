package models

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestUnionValueMarshalArchive(t *testing.T) {
	t.Run("menu button", func(t *testing.T) {
		checkUnionArchive[MenuButton](t, `{"type":"web_app","text":"Open","web_app":{"url":"https://example.test"}}`)
	})
	t.Run("boost source", func(t *testing.T) {
		checkUnionArchive[ChatBoostSource](t, `{"source":"premium","user":{"id":42,"first_name":"Ada"}}`)
	})
	t.Run("boost with nested source", func(t *testing.T) {
		checkUnionArchive[ChatBoost](t, `{"boost_id":"boost-1","add_date":1000,"expiration_date":2000,"source":{"source":"giveaway","giveaway_message_id":56,"prize_star_count":25}}`)
	})
	t.Run("chat member", func(t *testing.T) {
		checkUnionArchive[ChatMember](t, `{"status":"member","user":{"id":43,"first_name":"Sam"},"tag":"Helper"}`)
	})
	t.Run("background type", func(t *testing.T) {
		checkUnionArchive[BackgroundType](t, `{"type":"chat_theme","theme_name":"mint"}`)
	})
	t.Run("background with nested fill", func(t *testing.T) {
		checkUnionArchive[ChatBackground](t, `{"type":{"type":"fill","fill":{"type":"gradient","top_color":1048576,"bottom_color":255,"rotation_angle":45},"dark_theme_dimming":30}}`)
	})
	t.Run("background fill", func(t *testing.T) {
		checkUnionArchive[BackgroundFill](t, `{"type":"freeform_gradient","colors":[1048576,255,65280]}`)
	})
}

func checkUnionArchive[T any](t *testing.T, input string) {
	t.Helper()
	var original T
	if err := json.Unmarshal([]byte(input), &original); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name  string
		value any
	}{
		{name: "value", value: original},
		{name: "pointer", value: &original},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			var replay T
			if err := json.Unmarshal(encoded, &replay); err != nil {
				t.Fatalf("replay %s: %v", encoded, err)
			}
			if !reflect.DeepEqual(replay, original) {
				t.Fatalf("replay changed %T from input %s to archive %s", original, input, encoded)
			}
		})
	}
}

func TestUnionValueMarshalMissingPayload(t *testing.T) {
	for _, value := range []any{
		MenuButton{Type: MenuButtonTypeCommands},
		ChatBoostSource{Source: ChatBoostSourceTypePremium},
		ChatMember{Type: ChatMemberTypeMember},
		BackgroundType{Type: ChatBackgroundTypeWallpaper},
		BackgroundFill{Type: BackgroundFillTypeSolid},
	} {
		if _, err := json.Marshal(value); err == nil {
			t.Errorf("%T accepted a known discriminator without its payload", value)
		}
	}
}

func TestUnionValueMarshalDoesNotMutateVariants(t *testing.T) {
	webApp := &MenuButtonWebApp{Text: "Open"}
	premium := &ChatBoostSourcePremium{User: User{ID: 42}}
	member := &ChatMemberMember{User: &User{ID: 43}}
	solid := &BackgroundFillSolid{Color: 255}
	fill := &BackgroundTypeFill{Fill: BackgroundFill{Type: BackgroundFillTypeSolid, Solid: solid}}

	for _, wrapper := range []any{
		&MenuButton{Type: MenuButtonTypeWebApp, WebApp: webApp},
		&ChatBoostSource{Source: ChatBoostSourceTypePremium, ChatBoostSourcePremium: premium},
		&ChatMember{Type: ChatMemberTypeMember, Member: member},
		&ChatBackground{Type: BackgroundType{Type: ChatBackgroundTypeFill, Fill: fill}},
	} {
		if _, err := json.Marshal(wrapper); err != nil {
			t.Errorf("marshal %T: %v", wrapper, err)
		}
	}
	if webApp.Type != "" || premium.Source != "" || member.Status != "" || fill.Type != "" || solid.Type != "" {
		t.Errorf("marshal changed the payload discriminator: web app %q, boost %q, membership %q, background %q, fill %q",
			webApp.Type, premium.Source, member.Status, fill.Type, solid.Type)
	}
}

func TestUnionValueMarshalRestoresLegacyArchives(t *testing.T) {
	t.Run("menu button", func(t *testing.T) {
		checkLegacyUnionArchive[MenuButton](t,
			`{"Type":"web_app","Commands":null,"WebApp":{"type":"","text":"Open","web_app":{"url":"https://example.test"}},"Default":null}`,
			`{"type":"web_app","text":"Open","web_app":{"url":"https://example.test"}}`)
	})
	t.Run("boost with nested source", func(t *testing.T) {
		checkLegacyUnionArchive[ChatBoost](t,
			`{"boost_id":"boost-1","source":{"Source":"premium","ChatBoostSourcePremium":{"source":"premium","user":{"id":42,"first_name":"Ada"}},"ChatBoostSourceGiftCode":null,"ChatBoostSourceGiveaway":null}}`,
			`{"boost_id":"boost-1","source":{"source":"premium","user":{"id":42,"first_name":"Ada"}}}`)
	})
	t.Run("chat member", func(t *testing.T) {
		checkLegacyUnionArchive[ChatMember](t,
			`{"Type":"member","Owner":null,"Administrator":null,"Member":{"status":"member","user":{"id":43,"first_name":"Sam"},"tag":"Helper"},"Restricted":null,"Left":null,"Banned":null}`,
			`{"status":"member","user":{"id":43,"first_name":"Sam"},"tag":"Helper"}`)
	})
	t.Run("unknown chat member", func(t *testing.T) {
		checkLegacyUnionArchive[ChatMember](t,
			`{"Type":"future","Owner":null,"Administrator":null,"Member":null,"Restricted":null,"Left":null,"Banned":null}`,
			`{"status":"future"}`)
	})
	t.Run("background with nested fill", func(t *testing.T) {
		checkLegacyUnionArchive[ChatBackground](t,
			`{"type":{"Type":"fill","Fill":{"type":"fill","fill":{"type":"solid","color":255},"dark_theme_dimming":30},"Wallpaper":null,"Pattern":null,"Theme":null}}`,
			`{"type":{"type":"fill","fill":{"type":"solid","color":255},"dark_theme_dimming":30}}`)
	})
	t.Run("background fill", func(t *testing.T) {
		checkLegacyUnionArchive[BackgroundFill](t,
			`{"Type":"solid","Solid":{"type":"solid","color":255},"Gradient":null,"FreeformGradient":null}`,
			`{"type":"solid","color":255}`)
	})
}

func checkLegacyUnionArchive[T any](t *testing.T, legacy, canonical string) {
	t.Helper()
	var replay, want T
	if err := json.Unmarshal([]byte(legacy), &replay); err != nil {
		t.Fatalf("decode legacy archive: %v", err)
	}
	if err := json.Unmarshal([]byte(canonical), &want); err != nil {
		t.Fatalf("decode canonical model: %v", err)
	}
	if !reflect.DeepEqual(replay, want) {
		data, err := json.Marshal(replay)
		t.Fatalf("legacy %T payload was not restored: got %s (%v), want %s", replay, data, err, canonical)
	}
}

func TestUnionValueMarshalLegacySelection(t *testing.T) {
	t.Run("canonical fields win", func(t *testing.T) {
		checkLegacyUnionArchive[MenuButton](t,
			`{"type":"web_app","Type":"web_app","WebApp":{"type":"web_app","text":"Old","web_app":{"url":"https://old.example.test"}},"text":"Open","web_app":{"url":"https://example.test"}}`,
			`{"type":"web_app","text":"Open","web_app":{"url":"https://example.test"}}`)
	})
	t.Run("chat member with an unrelated type field", func(t *testing.T) {
		checkLegacyUnionArchive[ChatMember](t,
			`{"status":"member","user":{"id":43,"first_name":"Sam"},"type":{"source":"snapshot"}}`,
			`{"status":"member","user":{"id":43,"first_name":"Sam"}}`)
	})
	t.Run("lowercase type is not a legacy chat member discriminator", func(t *testing.T) {
		var member ChatMember
		data := `{"type":"member","Member":{"status":"member","user":{"id":43}}}`
		if err := json.Unmarshal([]byte(data), &member); err == nil {
			t.Fatal("chat member accepted lowercase type as the legacy discriminator")
		}
	})
	t.Run("wrong inner discriminator", func(t *testing.T) {
		var member ChatMember
		data := `{"Type":"member","Member":{"status":"kicked","user":{"id":43}}}`
		if err := json.Unmarshal([]byte(data), &member); err == nil {
			t.Fatal("legacy chat member accepted a payload for a different discriminator")
		}
	})
}

func TestUnionValueMarshalUnknownAndNil(t *testing.T) {
	for _, tc := range []struct {
		value any
		want  string
	}{
		{MenuButton{Type: "future"}, `{"type":"future"}`},
		{ChatBoostSource{Source: "future"}, `{"source":"future"}`},
		{ChatMember{Type: "future"}, `{"status":"future"}`},
		{BackgroundType{Type: "future"}, `{"type":"future"}`},
		{BackgroundFill{Type: "future"}, `{"type":"future"}`},
		{(*MenuButton)(nil), "null"},
		{(*ChatBoostSource)(nil), "null"},
		{(*ChatMember)(nil), "null"},
		{(*BackgroundType)(nil), "null"},
		{(*BackgroundFill)(nil), "null"},
	} {
		data, err := json.Marshal(tc.value)
		if err != nil || string(data) != tc.want {
			t.Errorf("marshal %T = %s, %v, want %s", tc.value, data, err, tc.want)
		}
	}
}
