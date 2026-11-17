package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Send /select command to the bot to see the example in action.

const maxRecentKeyboards = 1024

type keyboardID struct {
	chatID    int64
	messageID int
}

type selection [3]bool

type multiselect struct {
	selections map[keyboardID]selection
	recent     []keyboardID
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	menu := &multiselect{selections: make(map[keyboardID]selection)}

	b, err := bot.New(os.Getenv("EXAMPLE_TELEGRAM_BOT_TOKEN"), menu.options()...)
	if nil != err {
		// panics for the sake of simplicity.
		// you should handle this error properly in your code.
		panic(err)
	}

	b.Start(ctx)
}

func (m *multiselect) options() []bot.Option {
	return []bot.Option{
		// Process clicks in order, including the edits they send to Telegram.
		bot.WithWorkers(1),
		bot.WithNotAsyncHandlers(),
		bot.WithMessageTextHandler("/select", bot.MatchTypeExact, m.commandHandler),
		bot.WithCallbackQueryDataHandler("btn_", bot.MatchTypePrefix, m.callbackHandler),
	}
}

func (m *multiselect) callbackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	query := update.CallbackQuery
	message := query.Message.Message
	const expired = "This keyboard has expired. Send /select to start again."
	if message == nil {
		answerCallback(ctx, b, query.ID, expired)
		return
	}
	id := keyboardID{chatID: message.Chat.ID, messageID: message.ID}
	selected, hasKeyboard := m.selections[id]
	if !hasKeyboard {
		answerCallback(ctx, b, query.ID, expired)
		return
	}
	answerCallback(ctx, b, query.ID, "")

	switch query.Data {
	case "btn_opt1":
		selected[0] = !selected[0]
	case "btn_opt2":
		selected[1] = !selected[1]
	case "btn_opt3":
		selected[2] = !selected[2]
	case "btn_select":
		m.submitSelection(ctx, b, id, selected)
		return
	default:
		return
	}

	// Telegram can apply the edit even when its response is lost.
	m.selections[id] = selected
	_, err := b.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
		ChatID:      id.chatID,
		MessageID:   id.messageID,
		ReplyMarkup: buildKeyboard(selected),
	})
	if err == nil {
		return
	}
	log.Printf("edit keyboard: %v", err)
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: id.chatID,
		Text:   "Selection recorded, but the keyboard refresh could not be confirmed.",
	})
	if err != nil {
		log.Printf("send keyboard refresh notice: %v", err)
	}
}

func answerCallback(ctx context.Context, b *bot.Bot, queryID, text string) {
	_, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: queryID, Text: text})
	if err != nil {
		log.Printf("answer callback query: %v", err)
	}
}

func (m *multiselect) submitSelection(ctx context.Context, b *bot.Bot, id keyboardID, selected selection) {
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: id.chatID,
		Text:   fmt.Sprintf("Selected options: %v", selected),
	})
	if err != nil {
		log.Printf("send selection: %v", err)
		return
	}
	delete(m.selections, id)
	_, err = b.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: id.chatID, MessageID: id.messageID})
	if err != nil {
		log.Printf("delete keyboard: %v", err)
	}
}

func buildKeyboard(selected selection) models.ReplyMarkup {
	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: buttonText("Option 1", selected[0]), CallbackData: "btn_opt1"},
				{Text: buttonText("Option 2", selected[1]), CallbackData: "btn_opt2"},
				{Text: buttonText("Option 3", selected[2]), CallbackData: "btn_opt3"},
			}, {
				{Text: "Select", CallbackData: "btn_select"},
			},
		},
	}

	return kb
}

func buttonText(text string, opt bool) string {
	if opt {
		return "✅ " + text
	}

	return "❌ " + text
}

func (m *multiselect) commandHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	message, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        "Select multiple options",
		ReplyMarkup: buildKeyboard(selection{}),
	})
	if err != nil {
		log.Printf("send keyboard: %v", err)
		return
	}
	m.rememberKeyboard(keyboardID{chatID: message.Chat.ID, messageID: message.ID})
}

func (m *multiselect) rememberKeyboard(id keyboardID) {
	// Bound the state retained for keyboards that are never submitted.
	if len(m.recent) == maxRecentKeyboards {
		delete(m.selections, m.recent[0])
		m.recent = m.recent[1:]
	}
	m.recent = append(m.recent, id)
	m.selections[id] = selection{}
}
