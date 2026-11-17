package models

import (
	"encoding/json"
)

// ChatMemberUpdated https://core.telegram.org/bots/api#chatmemberupdated
type ChatMemberUpdated struct {
	Chat                    Chat            `json:"chat"`
	From                    User            `json:"from"`
	Date                    int             `json:"date"`
	OldChatMember           ChatMember      `json:"old_chat_member"`
	NewChatMember           ChatMember      `json:"new_chat_member"`
	InviteLink              *ChatInviteLink `json:"invite_link,omitempty"`
	ViaJoinRequest          bool            `json:"via_join_request,omitempty"`
	ViaChatFolderInviteLink bool            `json:"via_chat_folder_invite_link,omitempty"`
}

type ChatMemberType string

const (
	ChatMemberTypeOwner         ChatMemberType = "creator"
	ChatMemberTypeAdministrator ChatMemberType = "administrator"
	ChatMemberTypeMember        ChatMemberType = "member"
	ChatMemberTypeRestricted    ChatMemberType = "restricted"
	ChatMemberTypeLeft          ChatMemberType = "left"
	ChatMemberTypeBanned        ChatMemberType = "kicked"
)

// ChatMember https://core.telegram.org/bots/api#chatmember
type ChatMember struct {
	Type ChatMemberType

	Owner         *ChatMemberOwner
	Administrator *ChatMemberAdministrator
	Member        *ChatMemberMember
	Restricted    *ChatMemberRestricted
	Left          *ChatMemberLeft
	Banned        *ChatMemberBanned
}

func (c *ChatMember) UnmarshalJSON(data []byte) error {
	v := struct {
		Status ChatMemberType `json:"status"`
	}{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	var legacyType ChatMemberType
	if v.Status == "" {
		// Match the old Go field exactly so unrelated lowercase fields are ignored.
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			return err
		}
		if tag, ok := fields["Type"]; ok {
			if err := json.Unmarshal(tag, &legacyType); err != nil {
				return err
			}
			v.Status = legacyType
		}
	}
	if v.Status == "" {
		return missingDiscriminator("ChatMember")
	}

	c.Type = v.Status

	switch v.Status {
	case ChatMemberTypeOwner:
		c.Owner = &ChatMemberOwner{}
		return unmarshalChatMemberVariant(data, c.Owner, "Owner", legacyType)
	case ChatMemberTypeAdministrator:
		c.Administrator = &ChatMemberAdministrator{}
		return unmarshalChatMemberVariant(data, c.Administrator, "Administrator", legacyType)
	case ChatMemberTypeMember:
		c.Member = &ChatMemberMember{}
		return unmarshalChatMemberVariant(data, c.Member, "Member", legacyType)
	case ChatMemberTypeRestricted:
		c.Restricted = &ChatMemberRestricted{}
		return unmarshalChatMemberVariant(data, c.Restricted, "Restricted", legacyType)
	case ChatMemberTypeLeft:
		c.Left = &ChatMemberLeft{}
		return unmarshalChatMemberVariant(data, c.Left, "Left", legacyType)
	case ChatMemberTypeBanned:
		c.Banned = &ChatMemberBanned{}
		return unmarshalChatMemberVariant(data, c.Banned, "Banned", legacyType)
	}

	if legacyType != "" {
		return validateUnknownArchivedChatMember(data)
	}
	return nil
}

func unmarshalChatMemberVariant(data []byte, dest any, archivedField string, legacyType ChatMemberType) error {
	if legacyType != "" {
		archived, err := archivedUnionVariantField(data, archivedField, "status", string(legacyType))
		if err != nil {
			return err
		}
		if archived == nil {
			return missingDiscriminator("ChatMember")
		}
		data = archived
	}
	return json.Unmarshal(data, dest)
}

func validateUnknownArchivedChatMember(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	// The old Go wrapper wrote every variant field, even when all were nil.
	for _, name := range []string{"Owner", "Administrator", "Member", "Restricted", "Left", "Banned"} {
		if _, ok := fields[name]; !ok {
			return missingDiscriminator("ChatMember")
		}
	}
	return nil
}

func (c ChatMember) MarshalJSON() ([]byte, error) {
	switch c.Type {
	case ChatMemberTypeOwner:
		return marshalVariant("ChatMember", c.Type, c.Owner, func(v *ChatMemberOwner) { v.Status = c.Type })
	case ChatMemberTypeAdministrator:
		return marshalVariant("ChatMember", c.Type, c.Administrator, func(v *ChatMemberAdministrator) { v.Status = c.Type })
	case ChatMemberTypeMember:
		return marshalVariant("ChatMember", c.Type, c.Member, func(v *ChatMemberMember) { v.Status = c.Type })
	case ChatMemberTypeRestricted:
		return marshalVariant("ChatMember", c.Type, c.Restricted, func(v *ChatMemberRestricted) { v.Status = c.Type })
	case ChatMemberTypeLeft:
		return marshalVariant("ChatMember", c.Type, c.Left, func(v *ChatMemberLeft) { v.Status = c.Type })
	case ChatMemberTypeBanned:
		return marshalVariant("ChatMember", c.Type, c.Banned, func(v *ChatMemberBanned) { v.Status = c.Type })
	}

	return marshalUnknownVariant("ChatMember", "status", c.Type)
}

// ChatMemberOwner https://core.telegram.org/bots/api#chatmemberowner
type ChatMemberOwner struct {
	Status      ChatMemberType `json:"status"` // The member's status in the chat, always "creator"
	User        *User          `json:"user"`
	IsAnonymous bool           `json:"is_anonymous"`
	CustomTitle string         `json:"custom_title,omitempty"`
}

// ChatMemberAdministrator https://core.telegram.org/bots/api#chatmemberadministrator
type ChatMemberAdministrator struct {
	Status                  ChatMemberType `json:"status"` // The member's status in the chat, always "administrator"
	User                    User           `json:"user"`
	CanBeEdited             bool           `json:"can_be_edited"`
	IsAnonymous             bool           `json:"is_anonymous"`
	CanManageChat           bool           `json:"can_manage_chat"`
	CanDeleteMessages       bool           `json:"can_delete_messages"`
	CanManageVideoChats     bool           `json:"can_manage_video_chats"`
	CanRestrictMembers      bool           `json:"can_restrict_members"`
	CanPromoteMembers       bool           `json:"can_promote_members"`
	CanChangeInfo           bool           `json:"can_change_info"`
	CanInviteUsers          bool           `json:"can_invite_users"`
	CanPostMessages         bool           `json:"can_post_messages,omitempty"`
	CanEditMessages         bool           `json:"can_edit_messages,omitempty"`
	CanPinMessages          bool           `json:"can_pin_messages,omitempty"`
	CanPostStories          bool           `json:"can_post_stories"`
	CanEditStories          bool           `json:"can_edit_stories"`
	CanDeleteStories        bool           `json:"can_delete_stories"`
	CanManageTopics         bool           `json:"can_manage_topics,omitempty"`
	CanManageDirectMessages bool           `json:"can_manage_direct_messages,omitempty"`
	CanManageTags           bool           `json:"can_manage_tags,omitempty"`
	CanSendWelcomeMessages  bool           `json:"can_send_welcome_messages"`
	CustomTitle             string         `json:"custom_title,omitempty"`
}

// ChatMemberMember https://core.telegram.org/bots/api#chatmembermember
type ChatMemberMember struct {
	Status    ChatMemberType `json:"status"` // The member's status in the chat, always "member"
	User      *User          `json:"user"`
	UntilDate int            `json:"until_date,omitempty"`
	Tag       string         `json:"tag,omitempty"`
}

// ChatMemberRestricted https://core.telegram.org/bots/api#chatmemberrestricted
type ChatMemberRestricted struct {
	Status                ChatMemberType `json:"status"` // The member's status in the chat, always "restricted"
	User                  *User          `json:"user"`
	IsMember              bool           `json:"is_member"`
	CanSendMessages       bool           `json:"can_send_messages"`
	CanSendAudios         bool           `json:"can_send_audios"`
	CanSendDocuments      bool           `json:"can_send_documents"`
	CanSendPhotos         bool           `json:"can_send_photos"`
	CanSendVideos         bool           `json:"can_send_videos"`
	CanSendVideoNotes     bool           `json:"can_send_video_notes"`
	CanSendVoiceNotes     bool           `json:"can_send_voice_notes"`
	CanSendPolls          bool           `json:"can_send_polls"`
	CanSendOtherMessages  bool           `json:"can_send_other_messages"`
	CanAddWebPagePreviews bool           `json:"can_add_web_page_previews"`
	CanChangeInfo         bool           `json:"can_change_info"`
	CanInviteUsers        bool           `json:"can_invite_users"`
	CanPinMessages        bool           `json:"can_pin_messages"`
	CanManageTopics       bool           `json:"can_manage_topics,omitempty"`
	CanEditTag            bool           `json:"can_edit_tag,omitempty"`
	CanReactToMessages    bool           `json:"can_react_to_messages,omitempty"`
	UntilDate             int            `json:"until_date"`
	Tag                   string         `json:"tag,omitempty"`
}

// ChatMemberLeft https://core.telegram.org/bots/api#chatmemberleft
type ChatMemberLeft struct {
	Status ChatMemberType `json:"status"` // The member's status in the chat, always "left"
	User   *User          `json:"user"`
}

// ChatMemberBanned https://core.telegram.org/bots/api#chatmemberbanned
type ChatMemberBanned struct {
	Status    ChatMemberType `json:"status"` // The member's status in the chat, always "kicked"
	User      *User          `json:"user"`
	UntilDate int            `json:"until_date"`
}
