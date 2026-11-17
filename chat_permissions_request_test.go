package bot

import (
	"context"
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestChatPermissionMethodsPreserveOptionalPermissions(t *testing.T) {
	const basePermissions = `{"can_send_messages":true,"can_send_audios":false,"can_send_documents":false,"can_send_photos":false,"can_send_videos":false,"can_send_video_notes":false,"can_send_voice_notes":false,"can_pin_messages":true`

	for _, tc := range []struct {
		name         string
		manageTopics *bool
		editTag      *bool
		react        *bool
		optionalJSON string
	}{
		{name: "omitted"},
		{
			name:         "explicit false",
			manageTopics: False(),
			editTag:      False(),
			react:        False(),
			optionalJSON: `,"can_manage_topics":false,"can_edit_tag":false,"can_react_to_messages":false`,
		},
		{
			name:         "explicit true",
			manageTopics: True(),
			editTag:      True(),
			react:        True(),
			optionalJSON: `,"can_manage_topics":true,"can_edit_tag":true,"can_react_to_messages":true`,
		},
		{
			name:         "mixed",
			editTag:      False(),
			react:        True(),
			optionalJSON: `,"can_edit_tag":false,"can_react_to_messages":true`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			permissions := models.ChatPermissions{
				CanSendMessages:    true,
				CanPinMessages:     true,
				CanManageTopics:    tc.manageTopics,
				CanEditTag:         tc.editTag,
				CanReactToMessages: tc.react,
			}
			wantPermissions := basePermissions + tc.optionalJSON + "}"

			t.Run("RestrictChatMember", func(t *testing.T) {
				client := &httpClient{t: t, resp: "true", reqPath: "/bot/restrictChatMember", reqFields: map[string]string{
					"chat_id":                          "123",
					"user_id":                          "456",
					"permissions":                      wantPermissions,
					"use_independent_chat_permissions": "true",
				}}
				b := &Bot{client: client}
				ok, err := b.RestrictChatMember(context.Background(), &RestrictChatMemberParams{
					ChatID:                        123,
					UserID:                        456,
					Permissions:                   &permissions,
					UseIndependentChatPermissions: true,
				})
				if err != nil || !ok {
					t.Fatalf("RestrictChatMember() = %t, %v, want true, nil", ok, err)
				}
			})

			t.Run("SetChatPermissions", func(t *testing.T) {
				client := &httpClient{t: t, resp: "true", reqPath: "/bot/setChatPermissions", reqFields: map[string]string{
					"chat_id":     "123",
					"permissions": wantPermissions,
				}}
				b := &Bot{client: client}
				ok, err := b.SetChatPermissions(context.Background(), &SetChatPermissionsParams{
					ChatID:      123,
					Permissions: permissions,
				})
				if err != nil || !ok {
					t.Fatalf("SetChatPermissions() = %t, %v, want true, nil", ok, err)
				}
			})
		})
	}
}
