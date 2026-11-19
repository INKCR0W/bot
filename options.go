package bot

import (
	"strings"
	"time"
)

// Option is a function that configures a bot.
type Option func(b *Bot)

// WithCheckInitTimeout allows to redefine CheckInitTimeout
func WithCheckInitTimeout(timeout time.Duration) Option {
	return func(b *Bot) {
		b.checkInitTimeout = timeout
	}
}

// WithMiddlewares allows to set middlewares for each incoming request
func WithMiddlewares(middlewares ...Middleware) Option {
	return func(b *Bot) {
		b.middlewares = append(b.middlewares, middlewares...)
	}
}

// WithMessageTextHandler allows to set handler for incoming text messages
// Also you can use *bot.RegisterHandler function after bot creation
func WithMessageTextHandler(pattern string, matchType MatchType, handler HandlerFunc) Option {
	return func(b *Bot) {
		b.RegisterHandler(HandlerTypeMessageText, pattern, matchType, handler)
	}
}

// WithCallbackQueryDataHandler allows to set handler for incoming callback query
// Also you can use *bot.RegisterHandler function after bot creation
func WithCallbackQueryDataHandler(pattern string, matchType MatchType, handler HandlerFunc) Option {
	return func(b *Bot) {
		b.RegisterHandler(HandlerTypeCallbackQueryData, pattern, matchType, handler)
	}
}

// WithPhotoCaptionHandler allows to set handler for incoming photos with caption
// Also you can use *bot.RegisterHandler function after bot creation
func WithPhotoCaptionHandler(pattern string, matchType MatchType, handler HandlerFunc) Option {
	return func(b *Bot) {
		b.RegisterHandler(HandlerTypePhotoCaption, pattern, matchType, handler)
	}
}

// WithDefaultHandler allows to set default handler for incoming updates
func WithDefaultHandler(handler HandlerFunc) Option {
	return func(b *Bot) {
		b.defaultHandlerFunc = handler
	}
}

// WithDebug allows to enable debug mode. In debug mode, all requests and responses are logged by debug handler
func WithDebug() Option {
	return func(b *Bot) {
		b.isDebug = true
	}
}

// WithErrorsHandler allows to set handler for errors
func WithErrorsHandler(handler ErrorsHandler) Option {
	return func(b *Bot) {
		b.errorsHandler = handler
	}
}

// WithDebugHandler allows to set handler for debug messages
func WithDebugHandler(handler DebugHandler) Option {
	return func(b *Bot) {
		b.debugHandler = handler
	}
}

// WithHTTPClient allows to set custom http client
func WithHTTPClient(pollTimeout time.Duration, client HttpClient) Option {
	return func(b *Bot) {
		b.pollTimeout = pollTimeout
		b.client = client
	}
}

// WithServerURL sets the base URL of a custom Bot API server.
func WithServerURL(serverURL string) Option {
	return func(b *Bot) {
		b.url = strings.TrimRight(serverURL, "/")
	}
}

// WithSkipGetMe allows skip call GetMe on bot init
func WithSkipGetMe() Option {
	return func(b *Bot) {
		b.skipGetMe = true
	}
}

// WithAllowedUpdates allows to set custom params for getUpdates method
func WithAllowedUpdates(params AllowedUpdates) Option {
	return func(b *Bot) {
		b.allowedUpdates = params
	}
}

// WithUpdatesChannelCap sets the capacity of the queue between the update
// source and the workers. Start fetches the next batch only after the current
// one is dispatched, and a webhook request waits until a worker takes its
// update, so the capacity only bounds how many updates wait for a worker.
// New returns an error for a negative capacity.
func WithUpdatesChannelCap(cap int) Option {
	return func(b *Bot) {
		b.updatesChanCap = cap
	}
}

// WithWebhookSecretToken allows setting X-Telegram-Bot-Api-Secret-Token sent from Telegram servers
func WithWebhookSecretToken(webhookSecretToken string) Option {
	return func(b *Bot) {
		b.webhookSecretToken = webhookSecretToken
	}
}

// WithWorkers allows setting the number of workers that are processing the Updates channel
func WithWorkers(workers int) Option {
	return func(b *Bot) {
		b.workers = workers
	}
}

// UseTestEnvironment allows to use test environment
func UseTestEnvironment() Option {
	return func(b *Bot) {
		b.testEnvironment = true
	}
}

// WithNotAsyncHandlers runs each handler in its worker goroutine.
// A worker then takes the next update only after its handler returns, so a
// WebhookHandler request may wait for a busy worker.
func WithNotAsyncHandlers() Option {
	return func(b *Bot) {
		b.notAsyncHandlers = true
	}
}

// WithConfirmUpdatesAfterHandlers makes Start confirm a polled update only after
// its handler returns, including through runtime.Goexit, so an update whose
// handler was interrupted by cancellation is delivered again when polling
// resumes. The next batch is not requested until every handler of the current
// one returns: a slow handler delays every later update, and a handler must not
// wait for a later update. It has no effect on webhooks.
func WithConfirmUpdatesAfterHandlers() Option {
	return func(b *Bot) {
		b.confirmUpdatesAfterHandlers = true
	}
}

// WithInitialOffset sets the offset parameter of the first getUpdates request,
// that is the identifier of the first update to return. Unlike upstream, it is
// not incremented: to resume after update N, pass N+1. A negative offset starts
// at that position from the end of the queue and discards older updates. It is
// retried when the SDK can establish that the request never reached Telegram
// or that Telegram rejected it before applying the offset. If the outcome is
// uncertain, polling resumes from the earliest unconfirmed update.
func WithInitialOffset(offset int64) Option {
	return func(b *Bot) {
		b.nextUpdateOffset = offset
	}
}
