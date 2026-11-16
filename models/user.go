package models

import "encoding/json"

// UserProfilePhotos https://core.telegram.org/bots/api#userprofilephotos
type UserProfilePhotos struct {
	TotalCount int           `json:"total_count"`
	Photos     [][]PhotoSize `json:"photos"`
}

// User https://core.telegram.org/bots/api#user
type User struct {
	ID                        int64  `json:"id"`
	IsBot                     bool   `json:"is_bot"`
	FirstName                 string `json:"first_name,omitempty"`
	LastName                  string `json:"last_name,omitempty"`
	Username                  string `json:"username,omitempty"`
	LanguageCode              string `json:"language_code,omitempty"`
	IsPremium                 bool   `json:"is_premium,omitempty"`
	AddedToAttachmentMenu     bool   `json:"added_to_attachment_menu,omitempty"`
	CanJoinGroups             bool   `json:"can_join_groups,omitempty"`
	CanReadAllGroupMessages   bool   `json:"can_read_all_group_messages,omitempty"`
	SupportInlineQueries      bool   `json:"supports_inline_queries,omitempty"`
	CanConnectToBusiness      bool   `json:"can_connect_to_business,omitempty"`
	HasTopicsEnabled          bool   `json:"has_topics_enabled,omitempty"`
	AllowsUsersToCreateTopics bool   `json:"allows_users_to_create_topics,omitempty"`
	CanManageBots             bool   `json:"can_manage_bots,omitempty"`
	SupportsGuestQueries      bool   `json:"supports_guest_queries,omitempty"`
}

// archivedUser also reads the old SDK spelling of supports_inline_queries.
// Use it only for users inside a recognized legacy archive.
type archivedUser User

func (u *archivedUser) UnmarshalJSON(data []byte) error {
	var user User
	fields := struct {
		*User
		InlineQueries       json.RawMessage `json:"supports_inline_queries"`
		LegacyInlineQueries json.RawMessage `json:"support_inline_queries"`
	}{User: &user}
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	flag := fields.InlineQueries
	if flag == nil {
		flag = fields.LegacyInlineQueries
	}
	if flag != nil {
		if err := json.Unmarshal(flag, &user.SupportInlineQueries); err != nil {
			return err
		}
	}
	*u = archivedUser(user)
	return nil
}

// UserProfileAudios https://core.telegram.org/bots/api#userprofileaudios
type UserProfileAudios struct {
	TotalCount int     `json:"total_count"`
	Audios     []Audio `json:"audios"`
}
