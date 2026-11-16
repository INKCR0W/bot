package models

import (
	"encoding/json"
	"testing"
)

// TestUniqueGiftInfo_Text verifies the 10.3 text, entities and is_private fields decode.
func TestUniqueGiftInfo_Text(t *testing.T) {
	src := `{"gift":{"gift_id":"g","base_name":"b","name":"n","number":1},"origin":"transfer","text":"hi","entities":[{"type":"bold","offset":0,"length":2}],"is_private":true}`
	var info UniqueGiftInfo
	if err := json.Unmarshal([]byte(src), &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if info.Text != "hi" {
		t.Fatalf("text = %q", info.Text)
	}
	if len(info.Entities) != 1 || info.Entities[0].Type != "bold" {
		t.Fatalf("entities not decoded: %+v", info.Entities)
	}
	if !info.IsPrivate {
		t.Fatal("is_private not decoded")
	}
}

func TestOwnedGift_RoundTrip(t *testing.T) {
	for _, c := range []struct {
		tag  string
		data string
	}{
		{"regular", `{"type":"regular","gift":{"id":"gift-42","star_count":25},"owned_gift_id":"regular-42","send_date":1700000000}`},
		{"unique", `{"type":"unique","gift":{"gift_id":"gift-43","name":"collectible","number":3},"owned_gift_id":"unique-43","send_date":1700000001}`},
	} {
		t.Run(c.tag, func(t *testing.T) {
			assertTaggedRoundTrip[OwnedGift](t, c.data, c.tag)
		})
	}
}

func TestOwnedGift_LegacyArchive(t *testing.T) {
	const legacy = `{"type":"unique","OwnedGiftRegular":null,"OwnedGiftUnique":{"type":"","gift":{"gift_id":"gift-43"},"owned_gift_id":"unique-43","send_date":1700000001,"sender_user":{"id":42,"is_bot":true,"support_inline_queries":true}}}`
	gift := assertTaggedRoundTrip[OwnedGift](t, legacy, "unique")
	if gift.OwnedGiftUnique == nil || gift.OwnedGiftUnique.Type != OwnedGiftTypeUnique || gift.OwnedGiftUnique.OwnedGiftID != "unique-43" || gift.OwnedGiftUnique.Gift.GiftID != "gift-43" {
		t.Fatalf("legacy archive lost gift information: %+v", gift.OwnedGiftUnique)
	}
	if gift.OwnedGiftUnique.SenderUser == nil || !gift.OwnedGiftUnique.SenderUser.SupportInlineQueries {
		t.Fatal("legacy archive lost the sender's inline query capability")
	}
}

func TestOwnedGift_PrefersFlatFields(t *testing.T) {
	const payload = `{"type":"regular","gift":{"id":"real"},"send_date":1700000000,"OwnedGiftRegular":{"type":"regular","gift":{"id":"ignored"},"send_date":1},"OwnedGiftUnique":null}`
	gift := assertTaggedRoundTrip[OwnedGift](t, payload, "regular")
	if gift.OwnedGiftRegular == nil || gift.OwnedGiftRegular.Gift.ID != "real" || gift.OwnedGiftRegular.SendDate != 1700000000 {
		t.Fatalf("unknown nested fields replaced the flat gift: %+v", gift.OwnedGiftRegular)
	}
}

func TestOwnedGift_RequiresExactArchiveKeys(t *testing.T) {
	const payload = `{"type":"regular","ownedgiftregular":{"type":"regular","gift":{"id":"ignored"}},"OwnedGiftUnique":null}`
	var gift OwnedGift
	if err := json.Unmarshal([]byte(payload), &gift); err != nil {
		t.Fatal(err)
	}
	if gift.OwnedGiftRegular == nil || gift.OwnedGiftRegular.Gift.ID != "" {
		t.Fatalf("non-archive extension was unwrapped: %+v", gift.OwnedGiftRegular)
	}
}

func TestAcceptedGiftTypes_MarshalJSON(t *testing.T) {
	wire, err := json.Marshal(AcceptedGiftTypes{LimitedGifts: true})
	if err != nil {
		t.Fatal(err)
	}
	var flags map[string]bool
	if err := json.Unmarshal(wire, &flags); err != nil {
		t.Fatal(err)
	}
	if premium, ok := flags["premium_subscription"]; !ok || premium {
		t.Fatalf("expected premium_subscription=false, got %s", wire)
	}
	if fromChannels, ok := flags["gifts_from_channels"]; !ok || fromChannels {
		t.Fatalf("expected gifts_from_channels=false, got %s", wire)
	}
	if !flags["limited_gifts"] {
		t.Fatalf("expected limited_gifts=true, got %s", wire)
	}
}
