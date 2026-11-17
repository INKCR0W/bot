package models

import (
	"encoding/json"
)

type MenuButtonType string

const (
	MenuButtonTypeCommands MenuButtonType = "commands"
	MenuButtonTypeWebApp   MenuButtonType = "web_app"
	MenuButtonTypeDefault  MenuButtonType = "default"
)

type InputMenuButton interface {
	menuButtonTag()
}

// MenuButton https://core.telegram.org/bots/api#menubutton
type MenuButton struct {
	Type MenuButtonType

	Commands *MenuButtonCommands
	WebApp   *MenuButtonWebApp
	Default  *MenuButtonDefault
}

func (c *MenuButton) UnmarshalJSON(data []byte) error {
	v := struct {
		Type       MenuButtonType `json:"type"`
		LegacyType MenuButtonType `json:"Type"`
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
		return missingDiscriminator("MenuButton")
	}

	c.Type = v.Type

	switch v.Type {
	case MenuButtonTypeCommands:
		c.Commands = &MenuButtonCommands{}
		return unmarshalVariantWithArchive(data, c.Commands, "Commands", "type", string(v.LegacyType))
	case MenuButtonTypeWebApp:
		c.WebApp = &MenuButtonWebApp{}
		return unmarshalVariantWithArchive(data, c.WebApp, "WebApp", "type", string(v.LegacyType))
	case MenuButtonTypeDefault:
		c.Default = &MenuButtonDefault{}
		return unmarshalVariantWithArchive(data, c.Default, "Default", "type", string(v.LegacyType))
	}

	return nil
}

func (c MenuButton) MarshalJSON() ([]byte, error) {
	switch c.Type {
	case MenuButtonTypeCommands:
		return marshalVariant("MenuButton", c.Type, c.Commands, func(v *MenuButtonCommands) { v.Type = c.Type })
	case MenuButtonTypeWebApp:
		return marshalVariant("MenuButton", c.Type, c.WebApp, func(v *MenuButtonWebApp) { v.Type = c.Type })
	case MenuButtonTypeDefault:
		return marshalVariant("MenuButton", c.Type, c.Default, func(v *MenuButtonDefault) { v.Type = c.Type })
	}

	return marshalUnknownVariant("MenuButton", "type", c.Type)
}

// MenuButtonCommands https://core.telegram.org/bots/api#menubuttoncommands
type MenuButtonCommands struct {
	Type MenuButtonType `json:"type" rules:"required,equals:commands"`
}

func (MenuButtonCommands) menuButtonTag() {}

// MenuButtonWebApp https://core.telegram.org/bots/api#menubuttonwebapp
type MenuButtonWebApp struct {
	Type   MenuButtonType `json:"type" rules:"required,equals:web_app"`
	Text   string         `json:"text" rules:"required"`
	WebApp WebAppInfo     `json:"web_app" rules:"required"`
}

func (MenuButtonWebApp) menuButtonTag() {}

// MenuButtonDefault https://core.telegram.org/bots/api#menubuttondefault
type MenuButtonDefault struct {
	Type MenuButtonType `json:"type" rules:"required,equals:default"`
}

func (MenuButtonDefault) menuButtonTag() {}
