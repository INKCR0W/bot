package bot

import (
	"context"

	"github.com/go-telegram/bot/models"
)

func applyMiddlewares(h HandlerFunc, m ...Middleware) HandlerFunc {
	if len(m) < 1 {
		return h
	}
	wrapped := h
	for i := len(m) - 1; i >= 0; i-- {
		wrapped = m[i](wrapped)
	}
	return wrapped
}

// ProcessUpdate allows you to process update
func (b *Bot) ProcessUpdate(ctx context.Context, upd *models.Update) {
	h := b.findHandler(upd)

	r := applyMiddlewares(h, b.middlewares...)

	if b.notAsyncHandlers {
		r(ctx, b, upd)
		return
	}

	go r(ctx, b, upd)
}

func (b *Bot) findHandler(upd *models.Update) HandlerFunc {
	b.handlersMx.RLock()
	handlers := b.handlers
	b.handlersMx.RUnlock()

	if len(handlers) == 0 {
		return b.defaultHandlerFunc
	}
	b.tokenMx.RLock()
	username := b.username
	b.tokenMx.RUnlock()

	commands := commandMatchCache{botUsername: username}
	addressedHandlerIndex := -1
	addressedMayHaveChanged := false
	for i, h := range handlers {
		direct, addressed := h.match(upd, &commands)
		if direct {
			return h.handler
		}
		if addressed && addressedHandlerIndex == -1 {
			addressedHandlerIndex = i
		}
		if h.matchType == matchTypeFunc && addressedHandlerIndex != -1 {
			addressedMayHaveChanged = true
		}
	}
	if addressedHandlerIndex != -1 {
		if !addressedMayHaveChanged {
			return handlers[addressedHandlerIndex].handler
		}
		// Recheck the final update before dispatching a deferred command match.
		for _, h := range handlers {
			if h.matchesAddressedCommand(upd, &commands) {
				return h.handler
			}
		}
	}

	return b.defaultHandlerFunc
}
