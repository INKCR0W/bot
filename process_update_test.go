package bot

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
)

func Test_applyMiddlewares(t *testing.T) {
	h := func(ctx context.Context, bot *Bot, update *models.Update) {}

	middleware1 := func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, bot *Bot, update *models.Update) {
			next(ctx, bot, update)
		}
	}

	middleware2 := func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, bot *Bot, update *models.Update) {
			next(ctx, bot, update)
		}
	}

	wrapped := applyMiddlewares(h, middleware1, middleware2)
	if wrapped == nil {
		t.Fatal("Expected wrapped handler to be non-nil")
	}
}

func TestProcessUpdate(t *testing.T) {
	var called bool
	h := func(ctx context.Context, bot *Bot, update *models.Update) {
		called = true
	}

	bot := &Bot{
		defaultHandlerFunc: h,
		middlewares:        []Middleware{},
		notAsyncHandlers:   true,
	}

	ctx := context.Background()
	upd := &models.Update{Message: &models.Message{Text: "test"}}

	bot.ProcessUpdate(ctx, upd)
	if !called {
		t.Fatal("Expected default handler to be called")
	}
}

func TestProcessUpdate_WithMiddlewares(t *testing.T) {
	var called bool
	h := func(ctx context.Context, bot *Bot, update *models.Update) {
		called = true
	}

	middleware := func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, bot *Bot, update *models.Update) {
			next(ctx, bot, update)
		}
	}

	bot := &Bot{
		defaultHandlerFunc: h,
		middlewares:        []Middleware{middleware},
		notAsyncHandlers:   true,
	}

	ctx := context.Background()
	upd := &models.Update{Message: &models.Message{Text: "test"}}

	bot.ProcessUpdate(ctx, upd)
	if !called {
		t.Fatal("Expected default handler to be called")
	}
}

func TestProcessUpdate_WithMatchTypeFunc(t *testing.T) {
	var called string
	h1 := func(ctx context.Context, bot *Bot, update *models.Update) {
		called = "h1"
	}
	h2 := func(ctx context.Context, bot *Bot, update *models.Update) {
		called = "h2"
	}
	m := func(update *models.Update) bool {
		return update.CallbackQuery.GameShortName == "game"
	}

	bot := &Bot{
		defaultHandlerFunc: h1,
		notAsyncHandlers:   true,
	}

	bot.RegisterHandlerMatchFunc(m, h2)

	ctx := context.Background()
	upd := &models.Update{ID: 42, CallbackQuery: &models.CallbackQuery{ID: "test", GameShortName: "game"}}

	bot.ProcessUpdate(ctx, upd)
	if called != "h2" {
		t.Fatalf("Expected h2 handler to be called but %s handler was called", called)
	}
}

func TestProcessUpdate_CommandEntities(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bot123:test/getMe" {
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		_, err := io.WriteString(w, `{"ok":true,"result":{"id":123,"is_bot":true,"first_name":"Test","username":"ExampleBot"}}`)
		if err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()

	b, err := New("123:test", WithServerURL(server.URL), WithNotAsyncHandlers(), WithDefaultHandler(func(context.Context, *Bot, *models.Update) {}))
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name        string
		pattern     string
		prefix      string
		offset      int
		command     string
		matchType   MatchType
		handlerType HandlerType
		want        bool
	}{
		{"Chinese prefix", "start", "你好 ", 3, "/start", MatchTypeCommand, HandlerTypeMessageText, true},
		{"emoji prefix in caption", "start", "😀 ", 3, "/start", MatchTypeCommand, HandlerTypePhotoCaption, true},
		{"mentioned bot after emoji", "start", "😀 ", 3, "/start@eXAMPLEbOT", MatchTypeCommand, HandlerTypeMessageText, true},
		{"mentioned bot at start", "start", "", 0, "/start@ExampleBot", MatchTypeCommandStartOnly, HandlerTypePhotoCaption, true},
		{"mentioned bot away from start", "start", "😀 ", 3, "/start@ExampleBot", MatchTypeCommandStartOnly, HandlerTypePhotoCaption, false},
		{"other bot", "start", "", 0, "/start@OtherBot", MatchTypeCommand, HandlerTypeMessageText, false},
		{"explicit qualified pattern", "start@OtherBot", "", 0, "/start@OtherBot", MatchTypeCommand, HandlerTypeMessageText, true},
		{"missing command name", "", "", 0, "/@ExampleBot", MatchTypeCommand, HandlerTypeMessageText, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			matched := false
			id := b.RegisterHandler(test.handlerType, test.pattern, test.matchType, func(context.Context, *Bot, *models.Update) {
				matched = true
			})
			defer b.UnregisterHandler(id)

			data := test.prefix + test.command + " arguments"
			entities := []models.MessageEntity{{Type: models.MessageEntityTypeBotCommand, Offset: test.offset, Length: len(test.command)}}
			message := &models.Message{}
			if test.handlerType == HandlerTypePhotoCaption {
				message.Caption = data
				message.CaptionEntities = entities
			} else {
				message.Text = data
				message.Entities = entities
			}
			b.ProcessUpdate(context.Background(), &models.Update{Message: message})
			if matched != test.want {
				t.Errorf("matched %v, want %v", matched, test.want)
			}
		})
	}
}

func TestProcessUpdate_AddressedCommandFallback(t *testing.T) {
	b := &Bot{username: "ExampleBot", notAsyncHandlers: true}
	var matched string
	record := func(label string) HandlerFunc {
		return func(context.Context, *Bot, *models.Update) { matched = label }
	}
	b.RegisterHandler(HandlerTypeMessageText, "start", MatchTypeCommand, record("first bare"))
	b.RegisterHandler(HandlerTypeMessageText, "start", MatchTypeCommandStartOnly, record("second bare"))
	matchCalls := 0
	shouldMatch := false
	b.RegisterHandlerMatchFunc(func(*models.Update) bool {
		matchCalls++
		return shouldMatch
	}, record("custom"))
	explicitID := b.RegisterHandler(HandlerTypeMessageText, "start@ExampleBot", MatchTypeCommand, record("explicit"))
	prefixID := b.RegisterHandler(HandlerTypeMessageText, "/start@", MatchTypePrefix, record("prefix"))

	command := "/start@ExampleBot"
	message := &models.Message{
		Text:     command,
		Entities: []models.MessageEntity{{Type: models.MessageEntityTypeBotCommand, Length: len(command)}},
	}
	check := func(want string, wantMatchCalls int) {
		t.Helper()
		matched = ""
		matchCalls = 0
		b.ProcessUpdate(context.Background(), &models.Update{Message: message})
		if matched != want || matchCalls != wantMatchCalls {
			t.Errorf("matched %q with %d custom matcher calls, want %q with %d calls", matched, matchCalls, want, wantMatchCalls)
		}
	}
	check("explicit", 1)
	b.UnregisterHandler(explicitID)
	check("prefix", 1)
	b.UnregisterHandler(prefixID)
	check("first bare", 1)
	shouldMatch = true
	check("custom", 1)

	bare := "/start"
	message.Text += " " + bare
	message.Entities = append(message.Entities, models.MessageEntity{Type: models.MessageEntityTypeBotCommand, Offset: len(command) + 1, Length: len(bare)})
	check("first bare", 0)
}

func TestProcessUpdate_CommandCacheAfterCustomMatcher(t *testing.T) {
	for _, test := range []struct {
		name        string
		handlerType HandlerType
	}{
		{name: "text", handlerType: HandlerTypeMessageText},
		{name: "caption", handlerType: HandlerTypePhotoCaption},
	} {
		t.Run(test.name, func(t *testing.T) {
			b := &Bot{notAsyncHandlers: true}
			matched := ""
			handler := func(label string) HandlerFunc {
				return func(context.Context, *Bot, *models.Update) { matched = label }
			}
			b.defaultHandlerFunc = handler("default")
			b.RegisterHandler(HandlerTypeMessageText, "next", MatchTypeCommand, handler("premature text"))
			b.RegisterHandler(HandlerTypePhotoCaption, "next", MatchTypeCommand, handler("premature caption"))
			calls := 0
			b.RegisterHandlerMatchFunc(func(upd *models.Update) bool {
				calls++
				upd.Message.Text = "😀😀 /next"
				upd.Message.Entities[0].Offset = 5
				upd.Message.Caption = "好 /next"
				upd.Message.CaptionEntities[0].Offset = 2
				return false
			}, handler("custom"))
			b.RegisterHandler(test.handlerType, "next", MatchTypeCommand, handler("after mutation"))

			entity := models.MessageEntity{Type: models.MessageEntityTypeBotCommand, Offset: 3, Length: 5}
			b.ProcessUpdate(context.Background(), &models.Update{Message: &models.Message{
				Text:            "你好 /prev",
				Entities:        []models.MessageEntity{entity},
				Caption:         "😀 /prev",
				CaptionEntities: []models.MessageEntity{entity},
			}})
			if matched != "after mutation" || calls != 1 {
				t.Errorf("matched %q with %d custom matcher calls, want after mutation with one call", matched, calls)
			}
		})
	}
}

func TestProcessUpdate_AddressedCommandRecheckedAfterCustomMatcher(t *testing.T) {
	for _, test := range []struct {
		name      string
		keepOther bool
		want      string
	}{
		{name: "no remaining candidate", want: "default"},
		{name: "next valid candidate in registration order", keepOther: true, want: "first keep"},
	} {
		t.Run(test.name, func(t *testing.T) {
			b := &Bot{username: "ExampleBot", notAsyncHandlers: true}
			var matched string
			record := func(label string) HandlerFunc {
				return func(context.Context, *Bot, *models.Update) { matched = label }
			}
			b.defaultHandlerFunc = record("default")
			b.RegisterHandler(HandlerTypeMessageText, "remove", MatchTypeCommand, record("removed candidate"))
			b.RegisterHandler(HandlerTypeMessageText, "keep", MatchTypeCommandStartOnly, record("wrong offset"))
			b.RegisterHandler(HandlerTypeMessageText, "keep", MatchTypeCommand, record("first keep"))
			calls := 0
			b.RegisterHandlerMatchFunc(func(u *models.Update) bool {
				calls++
				if test.keepOther {
					u.Message.Entities = u.Message.Entities[1:]
				} else {
					u.Message.Entities = nil
				}
				return false
			}, record("custom"))
			b.RegisterHandler(HandlerTypeMessageText, "keep", MatchTypeCommand, record("second keep"))
			remove, keep := "/remove@ExampleBot", "/keep@ExampleBot"
			b.ProcessUpdate(context.Background(), &models.Update{Message: &models.Message{
				Text: remove + " " + keep,
				Entities: []models.MessageEntity{
					{Type: models.MessageEntityTypeBotCommand, Length: len(remove)},
					{Type: models.MessageEntityTypeBotCommand, Offset: len(remove) + 1, Length: len(keep)},
				},
			}})
			if matched != test.want || calls != 1 {
				t.Errorf("matched %q with %d custom matcher calls, want %q with one call", matched, calls, test.want)
			}
		})
	}
}

func BenchmarkFindHandler_Commands(b *testing.B) {
	const segment = "/other    "
	b.Run("400 entities and 100 handlers", func(b *testing.B) {
		bot := &Bot{username: "ExampleBot"}
		for i := 0; i < 100; i++ {
			bot.RegisterHandler(HandlerTypeMessageText, "unmatched", MatchTypeCommand, nil)
		}
		message := &models.Message{
			Text:     strings.Repeat(segment, 400),
			Entities: make([]models.MessageEntity, 400),
		}
		for i := range message.Entities {
			message.Entities[i] = models.MessageEntity{Type: models.MessageEntityTypeBotCommand, Offset: i * len(segment), Length: len("/other")}
		}
		update := &models.Update{Message: message}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			bot.findHandler(update)
		}
	})
}

func TestProcessUpdate_CommandIdentityAfterTokenChange(t *testing.T) {
	oldStarted := make(chan struct{})
	releaseOld := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var response string
		switch r.URL.Path {
		case "/bot123:initial/getMe":
			response = `{"ok":true,"result":{"id":123,"is_bot":true,"first_name":"Old","username":"OldBot"}}`
		case "/bot123:renewed/getMe":
			close(oldStarted)
			select {
			case <-releaseOld:
			case <-r.Context().Done():
				return
			}
			response = `{"ok":true,"result":{"id":123,"is_bot":true,"first_name":"Old","username":"OldBot"}}`
		case "/bot456:test/getMe":
			response = `{"ok":true,"result":{"id":456,"is_bot":true,"first_name":"New","username":"NewBot"}}`
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if _, err := io.WriteString(w, response); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	matched := false
	b, err := New("123:initial", WithServerURL(server.URL), WithSkipGetMe(), WithNotAsyncHandlers(),
		WithDefaultHandler(func(context.Context, *Bot, *models.Update) {}),
		WithMessageTextHandler("start", MatchTypeCommand, func(context.Context, *Bot, *models.Update) { matched = true }),
	)
	if err != nil {
		t.Fatal(err)
	}
	checkCommand := func(command string, want bool) {
		t.Helper()
		matched = false
		b.ProcessUpdate(ctx, &models.Update{Message: &models.Message{
			Text:     command,
			Entities: []models.MessageEntity{{Type: models.MessageEntityTypeBotCommand, Length: len(command)}},
		}})
		if matched != want {
			t.Errorf("%s matched %v, want %v", command, matched, want)
		}
	}
	checkCommand("/start@OldBot", false)
	if _, err := b.GetMe(ctx); err != nil {
		t.Fatal(err)
	}
	checkCommand("/start@OldBot", true)
	b.SetToken("123:renewed")
	checkCommand("/start@OldBot", true)

	oldDone := make(chan error, 1)
	go func() {
		_, err := b.GetMe(ctx)
		oldDone <- err
	}()
	select {
	case <-oldStarted:
	case err := <-oldDone:
		t.Fatalf("old GetMe returned before reaching server: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	b.SetToken("456:test")
	checkCommand("/start@OldBot", false)
	if _, err := b.GetMe(ctx); err != nil {
		t.Fatal(err)
	}
	close(releaseOld)
	if err := <-oldDone; err != nil {
		t.Fatal(err)
	}
	checkCommand("/start@NewBot", true)
	checkCommand("/start@OldBot", false)
}

func TestProcessUpdate_CommandIdentityAfterConcurrentGetMe(t *testing.T) {
	for _, test := range []struct {
		name        string
		newerFails  bool
		wantAddress string
	}{
		{name: "newer success wins", wantAddress: "NewBot"},
		{name: "earlier success still applies after failure", newerFails: true, wantAddress: "OldBot"},
	} {
		t.Run(test.name, func(t *testing.T) {
			oldStarted := make(chan struct{})
			releaseOld := make(chan struct{})
			var requests int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var response string
				if atomic.AddInt32(&requests, 1) == 1 {
					close(oldStarted)
					select {
					case <-releaseOld:
					case <-r.Context().Done():
						return
					}
					response = `{"ok":true,"result":{"id":123,"is_bot":true,"first_name":"Test","username":"OldBot"}}`
				} else if test.newerFails {
					response = `{"ok":false,"error_code":429,"description":"slow down"}`
				} else {
					response = `{"ok":true,"result":{"id":123,"is_bot":true,"first_name":"Test","username":"NewBot"}}`
				}
				if _, err := io.WriteString(w, response); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			b, err := New("123:test", WithServerURL(server.URL), WithSkipGetMe(), WithNotAsyncHandlers(),
				WithDefaultHandler(func(context.Context, *Bot, *models.Update) {}),
			)
			if err != nil {
				t.Fatal(err)
			}
			oldDone := make(chan error, 1)
			go func() {
				_, err := b.GetMe(ctx)
				oldDone <- err
			}()
			select {
			case <-oldStarted:
			case err := <-oldDone:
				t.Fatalf("old GetMe returned before reaching server: %v", err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			_, err = b.GetMe(ctx)
			if (err != nil) != test.newerFails {
				t.Fatalf("newer GetMe returned %v", err)
			}
			close(releaseOld)
			if err := <-oldDone; err != nil {
				t.Fatal(err)
			}

			matched := false
			b.RegisterHandler(HandlerTypeMessageText, "start", MatchTypeCommand, func(context.Context, *Bot, *models.Update) {
				matched = true
			})
			for _, address := range []string{"OldBot", "NewBot"} {
				matched = false
				command := "/start@" + address
				b.ProcessUpdate(ctx, &models.Update{Message: &models.Message{
					Text:     command,
					Entities: []models.MessageEntity{{Type: models.MessageEntityTypeBotCommand, Length: len(command)}},
				}})
				if want := address == test.wantAddress; matched != want {
					t.Errorf("%s matched %v, want %v", command, matched, want)
				}
			}
		})
	}
}

func TestProcessUpdate_MatcherUnregistersItself(t *testing.T) {
	var called []string
	b := &Bot{
		defaultHandlerFunc: func(context.Context, *Bot, *models.Update) { called = append(called, "default") },
		notAsyncHandlers:   true,
	}
	var id string
	id = b.RegisterHandlerMatchFunc(func(*models.Update) bool {
		b.UnregisterHandler(id)
		return true
	}, func(context.Context, *Bot, *models.Update) {
		called = append(called, "once")
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		b.ProcessUpdate(context.Background(), &models.Update{})
		b.ProcessUpdate(context.Background(), &models.Update{})
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("processing blocked after the matcher unregistered itself")
	}
	if len(called) != 2 || called[0] != "once" || called[1] != "default" {
		t.Errorf("handler order %v, want [once default]", called)
	}
}

func Test_findHandler(t *testing.T) {
	var called bool
	h := func(ctx context.Context, bot *Bot, update *models.Update) {
		called = true
	}

	bot := &Bot{
		defaultHandlerFunc: h,
	}

	// Register a handler
	bot.handlers = append(bot.handlers, handler{
		id:          "test",
		handlerType: HandlerTypeMessageText,
		matchType:   MatchTypeExact,
		pattern:     "test",
		handler:     h,
	})

	ctx := context.Background()
	upd := &models.Update{Message: &models.Message{Text: "test"}}

	handler := bot.findHandler(upd)
	handler(ctx, bot, upd)

	if !called {
		t.Fatal("Expected registered handler to be called")
	}
}

func Test_findPhotoCaptionHandler(t *testing.T) {
	var called bool
	h := func(ctx context.Context, bot *Bot, update *models.Update) {
		called = true
	}

	bot := &Bot{
		defaultHandlerFunc: h,
	}

	// Register a handler
	bot.handlers = append(bot.handlers, handler{
		id:          "test",
		handlerType: HandlerTypePhotoCaption,
		matchType:   MatchTypeExact,
		pattern:     "test",
		handler:     h,
	})

	ctx := context.Background()
	upd := &models.Update{Message: &models.Message{Caption: "test"}}

	handler := bot.findHandler(upd)
	handler(ctx, bot, upd)

	if !called {
		t.Fatal("Expected registered handler to be called")
	}
}

func Test_findHandler_Default(t *testing.T) {
	var called bool
	h := func(ctx context.Context, bot *Bot, update *models.Update) {
		called = true
	}

	bot := &Bot{
		defaultHandlerFunc: h,
	}

	ctx := context.Background()
	upd := &models.Update{Message: &models.Message{Text: "test"}}

	handler := bot.findHandler(upd)
	handler(ctx, bot, upd)

	if !called {
		t.Fatal("Expected default handler to be called")
	}
}
