package models

import (
	"encoding/json"
	"testing"
)

func TestTransactionPartner_UnmarshalJSON_User(t *testing.T) {
	src := `{"type":"user","user":{"id":42}}`

	m := TransactionPartner{}

	err := json.Unmarshal([]byte(src), &m)
	if err != nil {
		t.Fatal(err)
	}

	if m.Type != TransactionPartnerTypeUser {
		t.Fatalf("invalid type %s, expect %s", m.Type, TransactionPartnerTypeUser)
	}

	if m.User == nil {
		t.Fatal("invalid User")
	}

	if m.User.User.ID != 42 {
		t.Fatalf("invalid id %d, expect 42", m.User.User.ID)
	}
}

func TestTransactionPartner_UnmarshalJSON_Other(t *testing.T) {
	src := `{"type":"other"}`

	m := TransactionPartner{}

	err := json.Unmarshal([]byte(src), &m)
	if err != nil {
		t.Fatal(err)
	}

	if m.Type != TransactionPartnerTypeOther {
		t.Fatalf("invalid type %s, expect %s", m.Type, TransactionPartnerTypeOther)
	}
}

func TestTransactionPartner_UnmarshalJSON_Fragment(t *testing.T) {
	src := `{"type":"fragment","withdrawal_state":{"type":"pending"}}`

	m := TransactionPartner{}

	err := json.Unmarshal([]byte(src), &m)
	if err != nil {
		t.Fatal(err)
	}

	if m.Type != TransactionPartnerTypeFragment {
		t.Fatalf("invalid type %s, expect %s", m.Type, TransactionPartnerTypeFragment)
	}

	if m.Fragment == nil {
		t.Fatal("invalid Fragment")
	}

	if m.Fragment.WithdrawalState == nil {
		t.Fatal("invalid WithdrawalState")
	}

	if m.Fragment.WithdrawalState.Type != RevenueWithdrawalStateTypePending {
		t.Fatalf("invalid type %s, expect %s", m.Fragment.WithdrawalState.Type, RevenueWithdrawalStateTypePending)
	}
}

func TestRevenueWithdrawalState_UnmarshalJSON_Pending(t *testing.T) {
	src := `{"type":"pending"}`

	m := RevenueWithdrawalState{}

	err := json.Unmarshal([]byte(src), &m)
	if err != nil {
		t.Fatal(err)
	}

	if m.Type != RevenueWithdrawalStateTypePending {
		t.Fatalf("invalid type %s, expect %s", m.Type, RevenueWithdrawalStateTypePending)
	}

	if m.Pending == nil {
		t.Fatal("invalid Pending")
	}

	if m.Pending.Type != RevenueWithdrawalStateTypePending {
		t.Fatalf("invalid type %s, expect %s", m.Pending.Type, RevenueWithdrawalStateTypePending)
	}
}

func TestRevenueWithdrawalState_UnmarshalJSON_Succeeded(t *testing.T) {
	src := `{"type":"succeeded","date":1,"url":"xxx"}`

	m := RevenueWithdrawalState{}

	err := json.Unmarshal([]byte(src), &m)
	if err != nil {
		t.Fatal(err)
	}

	if m.Type != RevenueWithdrawalStateTypeSucceeded {
		t.Fatalf("invalid type %s, expect %s", m.Type, RevenueWithdrawalStateTypeSucceeded)
	}

	if m.Succeeded == nil {
		t.Fatal("invalid Succeeded")
	}

	if m.Succeeded.Type != RevenueWithdrawalStateTypeSucceeded {
		t.Fatalf("invalid type %s, expect %s", m.Pending.Type, RevenueWithdrawalStateTypeSucceeded)
	}

	if m.Succeeded.Date != 1 {
		t.Fatalf("invalid date %d, expect 1", m.Succeeded.Date)
	}

	if m.Succeeded.URL != "xxx" {
		t.Fatalf("invalid url %s, expect xxx", m.Succeeded.URL)
	}
}

func TestRevenueWithdrawalState_UnmarshalJSON_Failed(t *testing.T) {
	src := `{"type":"failed"}`

	m := RevenueWithdrawalState{}

	err := json.Unmarshal([]byte(src), &m)
	if err != nil {
		t.Fatal(err)
	}

	if m.Type != RevenueWithdrawalStateTypeFailed {
		t.Fatalf("invalid type %s, expect %s", m.Type, RevenueWithdrawalStateTypeFailed)
	}

	if m.Failed == nil {
		t.Fatal("invalid Failed")
	}

	if m.Failed.Type != RevenueWithdrawalStateTypeFailed {
		t.Fatalf("invalid type %s, expect %s", m.Pending.Type, RevenueWithdrawalStateTypeFailed)
	}
}

func TestTransactionPartner_RoundTrip(t *testing.T) {
	for _, c := range []struct {
		tag  string
		data string
	}{
		{"user", `{"type":"user","transaction_type":"invoice_payment","user":{"id":42},"invoice_payload":"invoice-42"}`},
		{"chat", `{"type":"chat","chat":{"id":-42,"type":"channel"},"gift":{"id":"gift-42"}}`},
		{"affiliate_program", `{"type":"affiliate_program","sponsor_user":{"id":42},"commission_per_mille":150}`},
		{"fragment", `{"type":"fragment","withdrawal_state":{"type":"succeeded","date":1700000000,"url":"https://example.com"}}`},
		{"telegram_ads", `{"type":"telegram_ads"}`},
		{"telegram_api", `{"type":"telegram_api","request_count":42}`},
		{"other", `{"type":"other"}`},
	} {
		t.Run(c.tag, func(t *testing.T) {
			assertTaggedRoundTrip[TransactionPartner](t, c.data, c.tag)
		})
	}
}

func TestTransactionPartnerUser_GiftPurchase(t *testing.T) {
	const src = `{"type":"user","transaction_type":"gift_purchase","user":{"id":42},"gift":{"id":"gift-42","star_count":25}}`
	partner := assertTaggedRoundTrip[TransactionPartner](t, src, "user")
	if partner.User == nil || partner.User.Gift == nil || partner.User.Gift.ID != "gift-42" || partner.User.Gift.StarCount != 25 {
		t.Fatalf("gift purchase did not retain gift info: %+v", partner.User)
	}
}

func TestTransactionPartner_LegacyArchive(t *testing.T) {
	const legacyUser = `{"Type":"user","user":{"type":"user","transaction_type":"invoice_payment","user":{"id":42,"is_bot":true,"support_inline_queries":true},"affiliate":{"affiliate_user":{"id":43,"is_bot":true,"support_inline_queries":true,"supports_inline_queries":false}},"invoice_payload":"invoice-42"}}`
	partner := assertTaggedRoundTrip[TransactionPartner](t, legacyUser, "user")
	if partner.User == nil || partner.User.User.ID != 42 || partner.User.InvoicePayload != "invoice-42" {
		t.Fatalf("legacy archive lost invoice details: %+v", partner.User)
	}
	if !partner.User.User.SupportInlineQueries {
		t.Fatal("legacy archive lost the user's inline query capability")
	}
	if partner.User.Affiliate == nil || partner.User.Affiliate.AffiliateUser == nil || partner.User.Affiliate.AffiliateUser.SupportInlineQueries {
		t.Fatal("canonical false must win for an archived affiliate user")
	}

	const legacyFragment = `{"Type":"fragment","fragment":{"type":"fragment","withdrawal_state":{"type":"succeeded","succeeded":{"type":"","date":1700000000,"url":"https://example.com"}}}}`
	partner = assertTaggedRoundTrip[TransactionPartner](t, legacyFragment, "fragment")
	if partner.Fragment == nil || partner.Fragment.WithdrawalState == nil {
		t.Fatal("legacy archive lost withdrawal state")
	}
	state := partner.Fragment.WithdrawalState.Succeeded
	if state == nil || state.Type != RevenueWithdrawalStateTypeSucceeded || state.Date != 1700000000 || state.URL != "https://example.com" {
		t.Fatalf("legacy archive lost withdrawal details: %+v", state)
	}

	const flat = `{"Type":"user","transaction_type":"invoice_payment","user":{"id":43},"invoice_payload":"invoice-43"}`
	partner = assertTaggedRoundTrip[TransactionPartner](t, flat, "user")
	if partner.User == nil || partner.User.User.ID != 43 {
		t.Fatalf("flat JSON with uppercase Type lost user data: %+v", partner.User)
	}
}

func TestTransactionPartner_LegacyAffiliateProgram(t *testing.T) {
	const archive = `{"Type":"affiliate_program","affiliate_program":{"type":"affiliate_program","sponsor_user":{"id":42,"is_bot":true,"support_inline_queries":true},"commission_per_mille":150}}`
	partner := assertTaggedRoundTrip[TransactionPartner](t, archive, "affiliate_program")
	if partner.AffiliateProgram == nil || partner.AffiliateProgram.SponsorUser == nil || !partner.AffiliateProgram.SponsorUser.SupportInlineQueries {
		t.Fatal("legacy archive lost the sponsor's inline query capability")
	}
}

func TestTransactionPartner_LegacyGiftID(t *testing.T) {
	const archived = `{"Type":"user","user":{"type":"","transaction_type":"gift_purchase","user":{"id":42,"is_bot":false},"gift":"gift-42"}}`
	partner := assertTaggedRoundTrip[TransactionPartner](t, archived, "user")
	if partner.User == nil || partner.User.Type != TransactionPartnerTypeUser || partner.User.Gift == nil || partner.User.Gift.ID != "gift-42" {
		t.Fatalf("legacy archive lost user or gift data: %+v", partner.User)
	}

	const invalidAPIResponse = `{"type":"user","transaction_type":"gift_purchase","user":{"id":42},"gift":"gift-42"}`
	if err := json.Unmarshal([]byte(invalidAPIResponse), &partner); err == nil {
		t.Fatal("accepted a string gift in a Telegram response")
	}
}

func TestRevenueWithdrawalState_RoundTrip(t *testing.T) {
	for _, c := range []struct {
		tag  string
		data string
	}{
		{"pending", `{"type":"pending"}`},
		{"succeeded", `{"type":"succeeded","date":1700000000,"url":"https://example.com"}`},
		{"failed", `{"type":"failed"}`},
	} {
		t.Run(c.tag, func(t *testing.T) {
			assertTaggedRoundTrip[RevenueWithdrawalState](t, c.data, c.tag)
		})
	}
}

func TestRevenueWithdrawalState_PrefersFlatFields(t *testing.T) {
	const payload = `{"type":"succeeded","date":1700000000,"url":"https://example.com/real","succeeded":{"type":"succeeded","date":1,"url":"https://example.com/ignored"}}`
	state := assertTaggedRoundTrip[RevenueWithdrawalState](t, payload, "succeeded")
	if state.Succeeded == nil || state.Succeeded.Date != 1700000000 || state.Succeeded.URL != "https://example.com/real" {
		t.Fatalf("unknown nested field replaced the flat withdrawal: %+v", state.Succeeded)
	}
}
