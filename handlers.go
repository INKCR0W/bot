package bot

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/go-telegram/bot/models"
)

type HandlerType int

const (
	HandlerTypeMessageText HandlerType = iota
	HandlerTypeCallbackQueryData
	HandlerTypeCallbackQueryGameShortName
	HandlerTypePhotoCaption
	HandlerTypeInlineQuery
	HandlerTypeWidgetQueryData
	HandlerTypeSharedWidgetQueryData
)

type MatchType int

const (
	MatchTypeExact MatchType = iota
	MatchTypePrefix
	MatchTypeContains
	// MatchTypeCommand searches for a command entity anywhere in the text or caption.
	// A bare pattern also matches a command addressed to this bot, after direct matches
	// have been checked and once GetMe has succeeded.
	MatchTypeCommand
	// MatchTypeCommandStartOnly requires the command entity to start at offset zero.
	MatchTypeCommandStartOnly

	matchTypeRegexp
	matchTypeFunc
)

type handler struct {
	id          string
	handlerType HandlerType
	matchType   MatchType
	handler     HandlerFunc

	pattern   string
	re        *regexp.Regexp
	matchFunc MatchFunc
}

func (h handler) match(update *models.Update, commands *commandMatchCache) (direct, addressed bool) {
	if h.matchType == matchTypeFunc {
		matched := h.matchFunc(update)
		// A custom matcher can change the update before later handlers inspect it.
		commands.text = commandSource{}
		commands.caption = commandSource{}
		return matched, false
	}

	data, entities, ok := getDataFromUpdate(update, h.handlerType)
	if !ok {
		return false, false
	}

	switch h.matchType {
	case MatchTypeExact:
		return h.matchExact(data), false
	case MatchTypePrefix:
		return h.matchPrefix(data), false
	case MatchTypeContains:
		return h.matchContains(data), false
	case MatchTypeCommand, MatchTypeCommandStartOnly:
		return h.matchCommand(commands.get(h.handlerType, data, entities))
	case matchTypeRegexp:
		return h.matchRegexp(data), false
	default:
		return false, false
	}
}

func getDataFromUpdate(update *models.Update, handlerType HandlerType) (data string, entities []models.MessageEntity, ok bool) {
	switch handlerType {
	case HandlerTypeMessageText:
		if update.Message == nil {
			return "", nil, false
		}
		data = update.Message.Text
		entities = update.Message.Entities
	case HandlerTypeCallbackQueryData:
		if update.CallbackQuery == nil {
			return "", nil, false
		}
		data = update.CallbackQuery.Data
	case HandlerTypeCallbackQueryGameShortName:
		if update.CallbackQuery == nil {
			return "", nil, false
		}
		data = update.CallbackQuery.GameShortName
	case HandlerTypePhotoCaption:
		if update.Message == nil {
			return "", nil, false
		}
		data = update.Message.Caption
		entities = update.Message.CaptionEntities
	case HandlerTypeInlineQuery:
		if update.InlineQuery == nil {
			return "", nil, false
		}
		data = update.InlineQuery.Query
	case HandlerTypeWidgetQueryData:
		if update.WidgetQuery == nil {
			return "", nil, false
		}
		data = update.WidgetQuery.Data
	case HandlerTypeSharedWidgetQueryData:
		if update.SharedWidgetQuery == nil {
			return "", nil, false
		}
		data = update.SharedWidgetQuery.Data
	}
	return data, entities, true
}

func (h handler) matchExact(data string) bool {
	return data == h.pattern
}

func (h handler) matchPrefix(data string) bool {
	return strings.HasPrefix(data, h.pattern)
}

func (h handler) matchContains(data string) bool {
	return strings.Contains(data, h.pattern)
}

func (h handler) matchRegexp(data string) bool {
	return h.re.Match([]byte(data))
}

type parsedCommand struct {
	value         string
	addressedName string
	atStart       bool
}

type commandSource struct {
	commands []parsedCommand
	isLoaded bool
}

type commandMatchCache struct {
	botUsername string
	text        commandSource
	caption     commandSource
}

func (c *commandMatchCache) get(handlerType HandlerType, data string, entities []models.MessageEntity) []parsedCommand {
	var source *commandSource
	switch handlerType {
	case HandlerTypeMessageText:
		source = &c.text
	case HandlerTypePhotoCaption:
		source = &c.caption
	default:
		return nil
	}
	if !source.isLoaded {
		source.commands = extractCommands(data, entities, c.botUsername)
		source.isLoaded = true
	}
	return source.commands
}

func extractCommands(data string, entities []models.MessageEntity, botUsername string) []parsedCommand {
	count := 0
	for _, entity := range entities {
		if entity.Type == models.MessageEntityTypeBotCommand {
			count++
		}
	}
	if count == 0 {
		return nil
	}

	commands := make([]parsedCommand, 0, count)
	offsets := utf16ByteOffsets(data)
	for _, entity := range entities {
		if entity.Type != models.MessageEntityTypeBotCommand {
			continue
		}
		value, ok := extractCommand(data, offsets, entity)
		if !ok {
			continue
		}
		command := parsedCommand{value: value, atStart: entity.Offset == 0}
		if botUsername != "" {
			name, target, qualified := strings.Cut(value, "@")
			if qualified && strings.EqualFold(target, botUsername) {
				command.addressedName = name
			}
		}
		commands = append(commands, command)
	}
	return commands
}

func extractCommand(data string, offsets []int, entity models.MessageEntity) (string, bool) {
	limit := len(data)
	if offsets != nil {
		limit = len(offsets) - 1
	}
	// Check the offset before subtracting, avoiding overflow in offset + length.
	if entity.Offset < 0 || entity.Offset > limit || entity.Length <= 1 || entity.Length > limit-entity.Offset {
		return "", false
	}
	start, end := entity.Offset, entity.Offset+entity.Length
	if offsets != nil {
		start, end = offsets[start], offsets[end]
	}
	if start < 0 || end < 0 || data[start] != '/' {
		return "", false
	}
	return data[start+1 : end], true
}

// utf16ByteOffsets maps UTF-16 code unit boundaries to byte indices.
// A nil map means the string is ASCII and both indices are identical.
func utf16ByteOffsets(data string) []int {
	for i := 0; i < len(data); i++ {
		if data[i] < utf8.RuneSelf {
			continue
		}
		offsets := make([]int, i, len(data)+1)
		for j := range offsets {
			offsets[j] = j
		}
		for j, r := range data[i:] {
			offsets = append(offsets, i+j)
			if r > 0xFFFF {
				// The middle of a surrogate pair has no matching byte boundary.
				offsets = append(offsets, -1)
			}
		}
		return append(offsets, len(data))
	}
	return nil
}

func (h handler) matchCommand(commands []parsedCommand) (direct, addressed bool) {
	for _, command := range commands {
		if h.matchType == MatchTypeCommandStartOnly && !command.atStart {
			continue
		}
		if command.value == h.pattern {
			return true, false
		}
		if command.addressedName != "" && command.addressedName == h.pattern {
			addressed = true
		}
	}
	return false, addressed
}

func (h handler) matchesAddressedCommand(update *models.Update, commands *commandMatchCache) bool {
	if h.matchType != MatchTypeCommand && h.matchType != MatchTypeCommandStartOnly {
		return false
	}
	data, entities, ok := getDataFromUpdate(update, h.handlerType)
	if !ok {
		return false
	}
	for _, command := range commands.get(h.handlerType, data, entities) {
		if h.matchType == MatchTypeCommandStartOnly && !command.atStart {
			continue
		}
		if command.addressedName != "" && command.addressedName == h.pattern {
			return true
		}
	}
	return false
}

func (b *Bot) RegisterHandlerMatchFunc(matchFunc MatchFunc, f HandlerFunc, m ...Middleware) string {
	return b.addHandler(handler{
		matchType: matchTypeFunc,
		matchFunc: matchFunc,
		handler:   applyMiddlewares(f, m...),
	})
}

func (b *Bot) RegisterHandlerRegexp(handlerType HandlerType, re *regexp.Regexp, f HandlerFunc, m ...Middleware) string {
	return b.addHandler(handler{
		handlerType: handlerType,
		matchType:   matchTypeRegexp,
		re:          re,
		handler:     applyMiddlewares(f, m...),
	})
}

// RegisterHandlerInlineQuery registers a handler for all inline queries, including
// empty queries. For matching query text, use RegisterHandler or RegisterHandlerRegexp
// with HandlerTypeInlineQuery. Returns an ID that can be passed to UnregisterHandler.
func (b *Bot) RegisterHandlerInlineQuery(f HandlerFunc, m ...Middleware) string {
	return b.RegisterHandler(HandlerTypeInlineQuery, "", MatchTypePrefix, f, m...)
}

func (b *Bot) RegisterHandler(handlerType HandlerType, pattern string, matchType MatchType, f HandlerFunc, m ...Middleware) string {
	return b.addHandler(handler{
		handlerType: handlerType,
		matchType:   matchType,
		pattern:     pattern,
		handler:     applyMiddlewares(f, m...),
	})
}

func (b *Bot) addHandler(h handler) string {
	h.id = RandomString(16)
	b.handlersMx.Lock()
	defer b.handlersMx.Unlock()
	// Appending leaves every entry visible to an existing snapshot unchanged.
	b.handlers = append(b.handlers, h)
	return h.id
}

// UnregisterHandler removes a handler from future update dispatches.
// A dispatch already selecting a handler can still use the previous registrations.
func (b *Bot) UnregisterHandler(id string) {
	b.handlersMx.Lock()
	defer b.handlersMx.Unlock()

	for i, h := range b.handlers {
		if h.id == id {
			// A matcher can still be iterating over the original entries.
			handlers := make([]handler, len(b.handlers)-1)
			copy(handlers, b.handlers[:i])
			copy(handlers[i:], b.handlers[i+1:])
			b.handlers = handlers
			return
		}
	}
}
