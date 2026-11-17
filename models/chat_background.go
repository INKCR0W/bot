package models

import (
	"encoding/json"
)

type BackgroundTypeType string

const (
	ChatBackgroundTypeFill      BackgroundTypeType = "fill"
	ChatBackgroundTypeWallpaper BackgroundTypeType = "wallpaper"
	ChatBackgroundTypePattern   BackgroundTypeType = "pattern"
	ChatBackgroundTypeChatTheme BackgroundTypeType = "chat_theme"
)

// ChatBackground https://core.telegram.org/bots/api#chatbackground
type ChatBackground struct {
	Type BackgroundType `json:"type"`
}

// BackgroundType https://core.telegram.org/bots/api#backgroundtype
type BackgroundType struct {
	Type      BackgroundTypeType
	Fill      *BackgroundTypeFill
	Wallpaper *BackgroundTypeWallpaper
	Pattern   *BackgroundTypePattern
	Theme     *BackgroundTypeChatTheme
}

func (cb *BackgroundType) UnmarshalJSON(data []byte) error {
	v := struct {
		Type       BackgroundTypeType `json:"type"`
		LegacyType BackgroundTypeType `json:"Type"`
	}{}

	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	if v.Type == "" {
		v.Type = v.LegacyType
	} else {
		v.LegacyType = ""
	}
	if v.Type == "" {
		return missingDiscriminator("BackgroundType")
	}

	cb.Type = v.Type

	switch v.Type {
	case ChatBackgroundTypeFill:
		return unmarshalVariantWithArchive(data, &cb.Fill, "Fill", "type", string(v.LegacyType))
	case ChatBackgroundTypeWallpaper:
		return unmarshalVariantWithArchive(data, &cb.Wallpaper, "Wallpaper", "type", string(v.LegacyType))
	case ChatBackgroundTypePattern:
		return unmarshalVariantWithArchive(data, &cb.Pattern, "Pattern", "type", string(v.LegacyType))
	case ChatBackgroundTypeChatTheme:
		return unmarshalVariantWithArchive(data, &cb.Theme, "Theme", "type", string(v.LegacyType))
	}

	return nil
}

func (cb BackgroundType) MarshalJSON() ([]byte, error) {
	switch cb.Type {
	case ChatBackgroundTypeFill:
		return marshalVariant("BackgroundType", cb.Type, cb.Fill, func(v *BackgroundTypeFill) { v.Type = cb.Type })
	case ChatBackgroundTypeWallpaper:
		return marshalVariant("BackgroundType", cb.Type, cb.Wallpaper, func(v *BackgroundTypeWallpaper) { v.Type = cb.Type })
	case ChatBackgroundTypePattern:
		return marshalVariant("BackgroundType", cb.Type, cb.Pattern, func(v *BackgroundTypePattern) { v.Type = cb.Type })
	case ChatBackgroundTypeChatTheme:
		return marshalVariant("BackgroundType", cb.Type, cb.Theme, func(v *BackgroundTypeChatTheme) { v.Type = cb.Type })
	}

	return marshalUnknownVariant("BackgroundType", "type", cb.Type)
}

// BackgroundTypeFill https://core.telegram.org/bots/api#backgroundtypefill
type BackgroundTypeFill struct {
	Type             BackgroundTypeType `json:"type"`
	Fill             BackgroundFill     `json:"fill"`
	DarkThemeDimming int                `json:"dark_theme_dimming"`
}

// BackgroundTypeWallpaper https://core.telegram.org/bots/api#backgroundtypewallpaper
type BackgroundTypeWallpaper struct {
	Type             BackgroundTypeType `json:"type"`
	Document         Document           `json:"document"`
	DarkThemeDimming int                `json:"dark_theme_dimming"`
	IsBlurred        bool               `json:"is_blurred,omitempty"`
	IsMoving         bool               `json:"is_moving,omitempty"`
}

// BackgroundTypePattern https://core.telegram.org/bots/api#backgroundtypepattern
type BackgroundTypePattern struct {
	Type       BackgroundTypeType `json:"type"`
	Document   Document           `json:"document"`
	Fill       BackgroundFill     `json:"fill"`
	Intensity  int                `json:"intensity"`
	IsInverted bool               `json:"is_inverted,omitempty"`
	IsMoving   bool               `json:"is_moving,omitempty"`
}

// BackgroundTypeChatTheme https://core.telegram.org/bots/api#backgroundtypechattheme
type BackgroundTypeChatTheme struct {
	Type      BackgroundTypeType `json:"type"`
	ThemeName string             `json:"theme_name"`
}

type BackgroundFillType string

const (
	BackgroundFillTypeSolid            BackgroundFillType = "solid"
	BackgroundFillTypeGradient         BackgroundFillType = "gradient"
	BackgroundFillTypeFreeformGradient BackgroundFillType = "freeform_gradient"
)

type BackgroundFill struct {
	Type             BackgroundFillType
	Solid            *BackgroundFillSolid
	Gradient         *BackgroundFillGradient
	FreeformGradient *BackgroundFillFreeformGradient
}

func (bf *BackgroundFill) UnmarshalJSON(data []byte) error {
	v := struct {
		Type       BackgroundFillType `json:"type"`
		LegacyType BackgroundFillType `json:"Type"`
	}{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	if v.Type == "" {
		v.Type = v.LegacyType
	} else {
		v.LegacyType = ""
	}
	if v.Type == "" {
		return missingDiscriminator("BackgroundFill")
	}

	bf.Type = v.Type

	switch v.Type {
	case BackgroundFillTypeSolid:
		bf.Solid = &BackgroundFillSolid{}
		return unmarshalVariantWithArchive(data, bf.Solid, "Solid", "type", string(v.LegacyType))
	case BackgroundFillTypeGradient:
		bf.Gradient = &BackgroundFillGradient{}
		return unmarshalVariantWithArchive(data, bf.Gradient, "Gradient", "type", string(v.LegacyType))
	case BackgroundFillTypeFreeformGradient:
		bf.FreeformGradient = &BackgroundFillFreeformGradient{}
		return unmarshalVariantWithArchive(data, bf.FreeformGradient, "FreeformGradient", "type", string(v.LegacyType))
	}

	return nil
}

func (bf BackgroundFill) MarshalJSON() ([]byte, error) {
	switch bf.Type {
	case BackgroundFillTypeSolid:
		return marshalVariant("BackgroundFill", bf.Type, bf.Solid, func(v *BackgroundFillSolid) { v.Type = bf.Type })
	case BackgroundFillTypeGradient:
		return marshalVariant("BackgroundFill", bf.Type, bf.Gradient, func(v *BackgroundFillGradient) { v.Type = bf.Type })
	case BackgroundFillTypeFreeformGradient:
		return marshalVariant("BackgroundFill", bf.Type, bf.FreeformGradient, func(v *BackgroundFillFreeformGradient) { v.Type = bf.Type })
	}

	return marshalUnknownVariant("BackgroundFill", "type", bf.Type)
}

// BackgroundFillSolid https://core.telegram.org/bots/api#backgroundfillsolid
type BackgroundFillSolid struct {
	Type  BackgroundFillType `json:"type"`
	Color int                `json:"color"`
}

// BackgroundFillGradient https://core.telegram.org/bots/api#backgroundfillgradient
type BackgroundFillGradient struct {
	Type          BackgroundFillType `json:"type"`
	TopColor      int                `json:"top_color"`
	BottomColor   int                `json:"bottom_color"`
	RotationAngle int                `json:"rotation_angle"`
}

// BackgroundFillFreeformGradient https://core.telegram.org/bots/api#backgroundfillfreeformgradient
type BackgroundFillFreeformGradient struct {
	Type   BackgroundFillType `json:"type"`
	Colors []int              `json:"colors"`
}
