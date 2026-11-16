package models

import (
	"encoding/json"
)

type TransactionPartnerType string

const (
	TransactionPartnerTypeUser             TransactionPartnerType = "user"
	TransactionPartnerTypeChat             TransactionPartnerType = "chat"
	TransactionPartnerTypeAffiliateProgram TransactionPartnerType = "affiliate_program"
	TransactionPartnerTypeFragment         TransactionPartnerType = "fragment"
	TransactionPartnerTypeTelegramAds      TransactionPartnerType = "telegram_ads"
	TransactionPartnerTypeTelegramApi      TransactionPartnerType = "telegram_api"
	TransactionPartnerTypeOther            TransactionPartnerType = "other"
)

// TransactionPartner https://core.telegram.org/bots/api#transactionpartner
type TransactionPartner struct {
	Type TransactionPartnerType

	User             *TransactionPartnerUser             `json:"user,omitempty"`
	Chat             *TransactionPartnerChat             `json:"chat,omitempty"`
	AffiliateProgram *TransactionPartnerAffiliateProgram `json:"affiliate_program,omitempty"`
	Fragment         *TransactionPartnerFragment         `json:"fragment,omitempty"`
	TelegramAds      *TransactionPartnerTelegramAds      `json:"telegram_ads,omitempty"`
	TelegramApi      *TransactionPartnerTelegramApi      `json:"telegram_api,omitempty"`
	Other            *TransactionPartnerOther            `json:"other,omitempty"`
}

func (m TransactionPartner) MarshalJSON() ([]byte, error) {
	switch m.Type {
	case TransactionPartnerTypeUser:
		return marshalVariant("TransactionPartner", m.Type, m.User, func(v *TransactionPartnerUser) { v.Type = m.Type })
	case TransactionPartnerTypeChat:
		return marshalVariant("TransactionPartner", m.Type, m.Chat, func(v *TransactionPartnerChat) { v.Type = m.Type })
	case TransactionPartnerTypeAffiliateProgram:
		return marshalVariant("TransactionPartner", m.Type, m.AffiliateProgram, func(v *TransactionPartnerAffiliateProgram) { v.Type = m.Type })
	case TransactionPartnerTypeFragment:
		return marshalVariant("TransactionPartner", m.Type, m.Fragment, func(v *TransactionPartnerFragment) { v.Type = m.Type })
	case TransactionPartnerTypeTelegramAds:
		return marshalVariant("TransactionPartner", m.Type, m.TelegramAds, func(v *TransactionPartnerTelegramAds) { v.Type = m.Type })
	case TransactionPartnerTypeTelegramApi:
		return marshalVariant("TransactionPartner", m.Type, m.TelegramApi, func(v *TransactionPartnerTelegramApi) { v.Type = m.Type })
	case TransactionPartnerTypeOther:
		return marshalVariant("TransactionPartner", m.Type, m.Other, func(v *TransactionPartnerOther) { v.Type = m.Type })
	}

	return marshalUnknownVariant("TransactionPartner", "type", m.Type)
}

func (m *TransactionPartner) UnmarshalJSON(data []byte) error {
	v := struct {
		Type       TransactionPartnerType `json:"type"`
		LegacyType TransactionPartnerType `json:"Type"`
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
		return missingDiscriminator("TransactionPartner")
	}

	decoded := TransactionPartner{Type: v.Type}
	var variant any
	switch v.Type {
	case TransactionPartnerTypeUser:
		decoded.User = &TransactionPartnerUser{}
		variant = decoded.User
	case TransactionPartnerTypeChat:
		decoded.Chat = &TransactionPartnerChat{}
		variant = decoded.Chat
	case TransactionPartnerTypeAffiliateProgram:
		decoded.AffiliateProgram = &TransactionPartnerAffiliateProgram{}
		variant = decoded.AffiliateProgram
	case TransactionPartnerTypeFragment:
		decoded.Fragment = &TransactionPartnerFragment{}
		variant = decoded.Fragment
	case TransactionPartnerTypeTelegramAds:
		decoded.TelegramAds = &TransactionPartnerTelegramAds{}
		variant = decoded.TelegramAds
	case TransactionPartnerTypeTelegramApi:
		decoded.TelegramApi = &TransactionPartnerTelegramApi{}
		variant = decoded.TelegramApi
	case TransactionPartnerTypeOther:
		decoded.Other = &TransactionPartnerOther{}
		variant = decoded.Other
	}

	var archived []byte
	if legacy && variant != nil {
		archived, err = archivedVariantField(data, string(v.Type), string(v.Type))
		if err != nil {
			return err
		}
		if archived != nil {
			data = archived
		}
	}
	switch {
	case archived != nil && decoded.User != nil:
		err = decoded.User.unmarshalArchive(data)
	case archived != nil && decoded.AffiliateProgram != nil:
		err = decoded.AffiliateProgram.unmarshalArchive(data)
	case variant != nil:
		err = json.Unmarshal(data, variant)
	}
	if err != nil {
		return err
	}

	*m = decoded
	return nil
}

// AffiliateInfo https://core.telegram.org/bots/api#affiliateinfo
type AffiliateInfo struct {
	AffiliateUser      *User `json:"affiliate_user,omitempty"`
	AffiliateChat      *Chat `json:"affiliate_chat,omitempty"`
	CommissionPerMille int   `json:"commission_per_mille"`
	Amount             int   `json:"amount"`
	NanostarAmount     int   `json:"nanostar_amount"`
}

// TransactionPartnerUser https://core.telegram.org/bots/api#transactionpartneruser
type TransactionPartnerUser struct {
	Type                        TransactionPartnerType `json:"type"`
	TransactionType             string                 `json:"transaction_type"`
	User                        User                   `json:"user"`
	Affiliate                   *AffiliateInfo         `json:"affiliate,omitempty"`
	InvoicePayload              string                 `json:"invoice_payload,omitempty"`
	SubscriptionPeriod          int                    `json:"subscription_period,omitempty"`
	PaidMedia                   []*PaidMedia           `json:"paid_media,omitempty"`
	PaidMediaPayload            string                 `json:"paid_media_payload,omitempty"`
	Gift                        *Gift                  `json:"gift,omitempty"`
	PremiumSubscriptionDuration int                    `json:"premium_subscription_duration,omitempty"`
}

func (m *TransactionPartnerUser) unmarshalArchive(data []byte) error {
	type userFields TransactionPartnerUser
	type affiliateFields struct {
		AffiliateInfo
		AffiliateUser *archivedUser `json:"affiliate_user"`
	}
	archive := struct {
		*userFields
		User      archivedUser     `json:"user"`
		Affiliate *affiliateFields `json:"affiliate"`
		Gift      json.RawMessage  `json:"gift"`
	}{userFields: (*userFields)(m)}
	if err := json.Unmarshal(data, &archive); err != nil {
		return err
	}
	m.User = User(archive.User)
	if archive.Affiliate != nil {
		archive.Affiliate.AffiliateInfo.AffiliateUser = (*User)(archive.Affiliate.AffiliateUser)
		m.Affiliate = &archive.Affiliate.AffiliateInfo
	}
	if len(archive.Gift) == 0 {
		return nil
	}
	if archive.Gift[0] == '"' {
		var id string
		if err := json.Unmarshal(archive.Gift, &id); err != nil {
			return err
		}
		if id != "" {
			m.Gift = &Gift{ID: id}
		}
		return nil
	}
	return json.Unmarshal(archive.Gift, &m.Gift)
}

// TransactionPartnerChat https://core.telegram.org/bots/api#transactionpartnerchat
type TransactionPartnerChat struct {
	Type TransactionPartnerType `json:"type"`
	Chat Chat                   `json:"chat"`
	Gift *Gift                  `json:"gift,omitempty"`
}

// TransactionPartnerAffiliateProgram https://core.telegram.org/bots/api#transactionpartneraffiliateprogram
type TransactionPartnerAffiliateProgram struct {
	Type               TransactionPartnerType `json:"type"`
	SponsorUser        *User                  `json:"sponsor_user,omitempty"`
	CommissionPerMille int                    `json:"commission_per_mille"`
}

func (m *TransactionPartnerAffiliateProgram) unmarshalArchive(data []byte) error {
	archive := struct {
		*TransactionPartnerAffiliateProgram
		SponsorUser *archivedUser `json:"sponsor_user"`
	}{TransactionPartnerAffiliateProgram: m}
	if err := json.Unmarshal(data, &archive); err != nil {
		return err
	}
	m.SponsorUser = (*User)(archive.SponsorUser)
	return nil
}

// TransactionPartnerFragment https://core.telegram.org/bots/api#transactionpartnerfragment
type TransactionPartnerFragment struct {
	Type            TransactionPartnerType  `json:"type"`
	WithdrawalState *RevenueWithdrawalState `json:"withdrawal_state,omitempty"`
}

// TransactionPartnerTelegramAds https://core.telegram.org/bots/api#transactionpartnertelegramads
type TransactionPartnerTelegramAds struct {
	Type TransactionPartnerType `json:"type"`
}

// TransactionPartnerTelegramApi https://core.telegram.org/bots/api#transactionpartnertelegramapi
type TransactionPartnerTelegramApi struct {
	Type         TransactionPartnerType `json:"type"`
	RequestCount int                    `json:"request_count"`
}

// TransactionPartnerOther https://core.telegram.org/bots/api#transactionpartnerother
type TransactionPartnerOther struct {
	Type TransactionPartnerType `json:"type"`
}

type RevenueWithdrawalStateType string

const (
	RevenueWithdrawalStateTypePending   RevenueWithdrawalStateType = "pending"
	RevenueWithdrawalStateTypeSucceeded RevenueWithdrawalStateType = "succeeded"
	RevenueWithdrawalStateTypeFailed    RevenueWithdrawalStateType = "failed"
)

// RevenueWithdrawalState https://core.telegram.org/bots/api#revenuewithdrawalstate
type RevenueWithdrawalState struct {
	Type RevenueWithdrawalStateType `json:"type"`

	Pending   *RevenueWithdrawalStatePending   `json:"pending,omitempty"`
	Succeeded *RevenueWithdrawalStateSucceeded `json:"succeeded,omitempty"`
	Failed    *RevenueWithdrawalStateFailed    `json:"failed,omitempty"`
}

func (m RevenueWithdrawalState) MarshalJSON() ([]byte, error) {
	switch m.Type {
	case RevenueWithdrawalStateTypePending:
		return marshalVariant("RevenueWithdrawalState", m.Type, m.Pending, func(v *RevenueWithdrawalStatePending) { v.Type = m.Type })
	case RevenueWithdrawalStateTypeSucceeded:
		return marshalVariant("RevenueWithdrawalState", m.Type, m.Succeeded, func(v *RevenueWithdrawalStateSucceeded) { v.Type = m.Type })
	case RevenueWithdrawalStateTypeFailed:
		return marshalVariant("RevenueWithdrawalState", m.Type, m.Failed, func(v *RevenueWithdrawalStateFailed) { v.Type = m.Type })
	}

	return marshalUnknownVariant("RevenueWithdrawalState", "type", m.Type)
}

func (m *RevenueWithdrawalState) UnmarshalJSON(data []byte) error {
	v := struct {
		Type      RevenueWithdrawalStateType `json:"type"`
		Pending   json.RawMessage            `json:"pending"`
		Succeeded json.RawMessage            `json:"succeeded"`
		Failed    json.RawMessage            `json:"failed"`
		Date      json.RawMessage            `json:"date"`
		URL       json.RawMessage            `json:"url"`
	}{}
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}

	if v.Type == "" {
		return missingDiscriminator("RevenueWithdrawalState")
	}

	decoded := RevenueWithdrawalState{Type: v.Type}
	var variant any
	var archived json.RawMessage
	switch v.Type {
	case RevenueWithdrawalStateTypePending:
		decoded.Pending = &RevenueWithdrawalStatePending{}
		variant, archived = decoded.Pending, v.Pending
	case RevenueWithdrawalStateTypeSucceeded:
		decoded.Succeeded = &RevenueWithdrawalStateSucceeded{}
		variant = decoded.Succeeded
		if v.Date == nil && v.URL == nil {
			archived = v.Succeeded
		}
	case RevenueWithdrawalStateTypeFailed:
		decoded.Failed = &RevenueWithdrawalStateFailed{}
		variant, archived = decoded.Failed, v.Failed
	}

	if variant != nil {
		if archived != nil {
			nested, err := archivedVariantField(data, string(v.Type), string(v.Type))
			if err != nil {
				return err
			}
			if nested != nil {
				data = nested
			}
		}
		if err := json.Unmarshal(data, variant); err != nil {
			return err
		}
	}

	*m = decoded
	return nil
}

// RevenueWithdrawalStatePending https://core.telegram.org/bots/api#revenuewithdrawalstatepending
type RevenueWithdrawalStatePending struct {
	Type RevenueWithdrawalStateType `json:"type"`
}

// RevenueWithdrawalStateSucceeded https://core.telegram.org/bots/api#revenuewithdrawalstatesucceeded
type RevenueWithdrawalStateSucceeded struct {
	Type RevenueWithdrawalStateType `json:"type"`
	Date int                        `json:"date"`
	URL  string                     `json:"url"`
}

// RevenueWithdrawalStateFailed https://core.telegram.org/bots/api#revenuewithdrawalstatefailed
type RevenueWithdrawalStateFailed struct {
	Type RevenueWithdrawalStateType `json:"type"`
}

// StarTransaction https://core.telegram.org/bots/api#startransaction
type StarTransaction struct {
	ID             string              `json:"id"`
	Amount         int                 `json:"amount"`
	NanostarAmount int                 `json:"nanostar_amount,omitempty"`
	Date           int                 `json:"date"`
	Source         *TransactionPartner `json:"source,omitempty"`
	Receiver       *TransactionPartner `json:"receiver,omitempty"`
}

// StarTransactions https://core.telegram.org/bots/api#startransactions
type StarTransactions struct {
	Transactions []StarTransaction `json:"transactions"`
}

// StarAmount https://core.telegram.org/bots/api#staramount
type StarAmount struct {
	Amount         int `json:"amount"`
	NanostarAmount int `json:"nanostar_amount,omitempty"`
}
