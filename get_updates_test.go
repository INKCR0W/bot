package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
)

type getUpdatesClientFunc func(*http.Request) (*http.Response, error)

func (f getUpdatesClientFunc) Do(req *http.Request) (*http.Response, error) {
	// keep the body readable for req.FormValue
	body, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	return f(req)
}

// runGetUpdates polls like Start does and acts as the worker: it records every
// queued update and signals that it was dispatched.
func runGetUpdates(ctx context.Context, b *Bot) []*models.Update {
	queue := make(chan queuedUpdate)
	var delivered []*models.Update
	done := make(chan struct{})
	go func() {
		defer close(done)
		for item := range queue {
			delivered = append(delivered, item.update)
			item.claim.take()
		}
	}()
	b.getUpdates(ctx, queue)
	close(queue)
	<-done
	return delivered
}

func getUpdatesJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// One undecodable update must not poison the batch: the others are delivered,
// the offset moves past it and the failure is reported with the update id.
func Test_getUpdates_skipsUndecodableUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	batch := `{"ok":true,"result":[
		{"update_id":100,"message":{"message_id":1,"text":"first"}},
		{"update_id":101,"message":"not an object"},
		{"update_id":102,"message":{"message_id":2,"text":"third"}}
	]}`

	var calls int32
	var errs []string
	b := &Bot{
		token:         "XXX",
		errorsHandler: func(handlerErr error) { errs = append(errs, handlerErr.Error()) },
		debugHandler:  func(string, ...any) {},
		client: getUpdatesClientFunc(func(*http.Request) (*http.Response, error) {
			if atomic.AddInt32(&calls, 1) > 1 {
				cancel()
				return nil, ctx.Err()
			}
			return getUpdatesJSONResponse(batch), nil
		}),
	}

	delivered := runGetUpdates(ctx, b)

	if got := atomic.LoadInt64(&b.nextUpdateOffset); got != 103 {
		t.Fatalf("offset must move past the bad update, nextUpdateOffset=%d", got)
	}
	if len(delivered) != 2 {
		t.Fatalf("expected 2 delivered updates, got %d", len(delivered))
	}
	if first, third := delivered[0], delivered[1]; first.ID != 100 || third.ID != 102 {
		t.Fatalf("expected updates 100 and 102, got %d and %d", first.ID, third.ID)
	}
	if len(errs) != 1 || !strings.Contains(errs[0], "101") || !strings.Contains(errs[0], "not an object") {
		t.Fatalf("undecodable update must be reported once with its id and raw payload, got %v", errs)
	}
}

// An update whose id cannot be read leaves the offset where it is, so the very same
// batch comes back on the next request. Without a backoff that is a tight loop.
func Test_getUpdates_backsOffWhenUpdateIDUnreadable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	batch := `{"ok":true,"result":[{"update_id":"abc","message":"x"}]}`

	var calls int
	var firstCallAt time.Time
	var gap time.Duration

	b := &Bot{
		token:         "XXX",
		errorsHandler: func(error) {},
		debugHandler:  func(string, ...any) {},
		client: getUpdatesClientFunc(func(*http.Request) (*http.Response, error) {
			calls++
			switch calls {
			case 1:
				firstCallAt = time.Now()
			case 2:
				gap = time.Since(firstCallAt)
				cancel()
				return nil, ctx.Err()
			}
			return getUpdatesJSONResponse(batch), nil
		}),
	}

	runGetUpdates(ctx, b)

	if gap < 100*time.Millisecond {
		t.Fatalf("expected a backoff before the next request, got %v", gap)
	}
}

// pollGaps runs getUpdates against a client that answers every call with batch and
// returns the delays between consecutive requests, cancelling after calls requests.
func pollGaps(t *testing.T, batch string, calls int) []time.Duration {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var seen int
	var last time.Time
	var gaps []time.Duration

	b := &Bot{
		token:         "XXX",
		errorsHandler: func(error) {},
		debugHandler:  func(string, ...any) {},
		client: getUpdatesClientFunc(func(*http.Request) (*http.Response, error) {
			seen++
			if seen > 1 {
				gaps = append(gaps, time.Since(last))
			}
			last = time.Now()
			if seen == calls {
				cancel()
				return nil, ctx.Err()
			}
			return getUpdatesJSONResponse(batch), nil
		}),
	}

	runGetUpdates(ctx, b)

	return gaps
}

// While the offset stays stuck on the same unreadable batch the backoff must grow
// like it does for failed requests, not sit at the first step forever.
func Test_getUpdates_backoffGrowsWhileOffsetStuck(t *testing.T) {
	gaps := pollGaps(t, `{"ok":true,"result":[{"update_id":"abc","message":"x"}]}`, 4)

	if len(gaps) != 3 {
		t.Fatalf("expected 3 gaps, got %v", gaps)
	}
	for i, want := range []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond} {
		if gaps[i] < want {
			t.Fatalf("gap %d must be at least %v, got %v (all: %v)", i, want, gaps[i], gaps)
		}
	}
}

// Several unreadable elements in one batch are one stuck poll, not several: the
// backoff takes a single step per request.
func Test_getUpdates_backoffStepsOncePerBatch(t *testing.T) {
	batch := `{"ok":true,"result":[{"update_id":"a"},{"update_id":"b"},{"update_id":"c"},{"update_id":"d"}]}`
	gaps := pollGaps(t, batch, 2)

	if len(gaps) != 1 || gaps[0] >= 200*time.Millisecond {
		t.Fatalf("expected a single 100ms step, got %v", gaps)
	}
}

// When a later element of the batch advances the offset the unreadable one can never
// come back, so there is nothing to back off from.
func Test_getUpdates_noBackoffWhenLaterUpdateAdvancesOffset(t *testing.T) {
	batch := `{"ok":true,"result":[{"update_id":"abc","message":"x"},{"update_id":50,"message":{"message_id":1}}]}`
	gaps := pollGaps(t, batch, 2)

	if len(gaps) != 1 || gaps[0] >= 50*time.Millisecond {
		t.Fatalf("expected the next request without delay, got %v", gaps)
	}
}

// A null element decodes into an empty Update without an error. It must not be
// delivered and must not drag the offset back to zero.
func Test_getUpdates_ignoresUpdateWithoutID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	batch := `{"ok":true,"result":[{"update_id":500,"message":{"message_id":1}},null,{}]}`

	var calls int32
	b := &Bot{
		token:         "XXX",
		errorsHandler: func(error) {},
		debugHandler:  func(string, ...any) {},
		client: getUpdatesClientFunc(func(*http.Request) (*http.Response, error) {
			if atomic.AddInt32(&calls, 1) > 1 {
				cancel()
				return nil, ctx.Err()
			}
			return getUpdatesJSONResponse(batch), nil
		}),
	}

	delivered := runGetUpdates(ctx, b)

	if got := atomic.LoadInt64(&b.nextUpdateOffset); got != 501 {
		t.Fatalf("offset must stay after the last real update, nextUpdateOffset=%d", got)
	}
	if len(delivered) != 1 {
		t.Fatalf("expected only the real update to be delivered, got %d", len(delivered))
	}
}

func waitForUpdateTest[T any](t *testing.T, ch <-chan T, event string) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", event)
		var zero T
		return zero
	}
}

// A batch is confirmed once its asynchronous handlers start, so a handler can
// wait for an update from a later batch.
func TestBot_StartPollsWhileHandlerRuns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	secondHandled := make(chan struct{})
	firstSawSecond := make(chan bool, 1)
	offsets := make(chan string, 3)
	var calls int
	b, err := New("xxx", WithSkipGetMe(),
		WithDefaultHandler(func(_ context.Context, _ *Bot, update *models.Update) {
			if update.ID == 2 {
				close(secondHandled)
				return
			}
			select {
			case <-secondHandled:
				firstSawSecond <- true
			case <-time.After(2 * time.Second):
				firstSawSecond <- false
			}
		}),
		WithHTTPClient(time.Second, getUpdatesClientFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			offsets <- req.FormValue("offset")
			switch calls {
			case 1:
				return getUpdatesJSONResponse(`{"ok":true,"result":[{"update_id":1}]}`), nil
			case 2:
				return getUpdatesJSONResponse(`{"ok":true,"result":[{"update_id":2}]}`), nil
			}
			<-req.Context().Done()
			return nil, req.Context().Err()
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		b.Start(ctx)
	}()
	if !waitForUpdateTest(t, firstSawSecond, "first handler to finish") {
		t.Error("the second update was not dispatched while the first handler was running")
	}
	cancel()
	waitForUpdateTest(t, stopped, "polling to stop")
	if first, second := <-offsets, <-offsets; first != "1" || second != "2" {
		t.Errorf("poll offsets = %q, %q, want 1, 2", first, second)
	}
}

// Cancellation confirms the updates before the first undispatched one, so
// polling resumes there without replaying the handled update.
func TestBot_StartResumesFromFirstUndispatchedUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resumeCtx, cancelResume := context.WithCancel(context.Background())
	defer cancelResume()

	firstStarted := make(chan struct{})
	offsets := make(chan string, 4)
	var handled []int64
	var calls int
	var restarting bool
	b, err := New("xxx", WithSkipGetMe(), WithNotAsyncHandlers(),
		WithDefaultHandler(func(ctx context.Context, _ *Bot, update *models.Update) {
			handled = append(handled, update.ID)
			if !restarting && update.ID == 40 {
				close(firstStarted)
				<-ctx.Done()
			}
		}),
		WithHTTPClient(time.Second, getUpdatesClientFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			offsets <- req.FormValue("offset")
			if calls == 1 {
				if req.FormValue("offset") == "41" {
					return getUpdatesJSONResponse(`{"ok":true,"result":[{"update_id":42}]}`), nil
				}
				return getUpdatesJSONResponse(`{"ok":true,"result":[{"update_id":40},{"update_id":42}]}`), nil
			}
			<-req.Context().Done()
			return nil, req.Context().Err()
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		b.Start(ctx)
	}()
	waitForUpdateTest(t, firstStarted, "first handler to start")
	cancel()
	waitForUpdateTest(t, stopped, "polling to stop")
	if got := waitForUpdateTest(t, offsets, "first poll"); got != "1" {
		t.Errorf("first poll offset = %q, want 1", got)
	}

	restarting = true
	calls = 0
	resumed := make(chan struct{})
	go func() {
		defer close(resumed)
		b.Start(resumeCtx)
	}()
	if got := waitForUpdateTest(t, offsets, "resumed poll"); got != "41" {
		t.Errorf("resumed poll offset = %q, want 41", got)
	}
	if got := waitForUpdateTest(t, offsets, "poll after the resumed batch"); got != "43" {
		t.Errorf("offset after the resumed batch = %q, want 43", got)
	}
	cancelResume()
	waitForUpdateTest(t, resumed, "resumed polling to stop")
	if want := []int64{40, 42}; !reflect.DeepEqual(handled, want) {
		t.Errorf("handled updates = %v, want %v", handled, want)
	}
}

func TestBot_StartKeepsOffsetWhenLastDispatchedHandlerCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	offsets := make(chan string, 2)
	var calls int
	b, err := New("xxx", WithSkipGetMe(), WithNotAsyncHandlers(),
		WithDefaultHandler(func(context.Context, *Bot, *models.Update) {
			cancel()
		}),
		WithErrorsHandler(func(error) {
			// The malformed record keeps the poller from reading the dispatch
			// signal until after the valid update's handler cancels the run.
			<-ctx.Done()
		}),
		WithHTTPClient(time.Second, getUpdatesClientFunc(func(req *http.Request) (*http.Response, error) {
			offsets <- req.FormValue("offset")
			calls++
			if calls == 1 {
				return getUpdatesJSONResponse(`{"ok":true,"result":[{"update_id":42},{"update_id":"invalid"}]}`), nil
			}
			return nil, context.Canceled
		})))
	if err != nil {
		t.Fatal(err)
	}

	b.Start(ctx)
	if got := waitForUpdateTest(t, offsets, "initial poll"); got != "1" {
		t.Errorf("initial offset = %q, want 1", got)
	}

	b.Start(context.Background())
	if got := waitForUpdateTest(t, offsets, "poll after restart"); got != "43" {
		t.Errorf("offset after restart = %q, want 43", got)
	}
}

func TestBot_StartWaitsForDispatchedHandlers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	release := make(chan struct{})
	var calls int
	b, err := New("xxx", WithSkipGetMe(),
		WithDefaultHandler(func(context.Context, *Bot, *models.Update) {
			close(started)
			// The handler ignores cancellation on purpose.
			<-release
		}),
		WithHTTPClient(time.Second, getUpdatesClientFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return getUpdatesJSONResponse(`{"ok":true,"result":[{"update_id":1}]}`), nil
			}
			<-req.Context().Done()
			return nil, req.Context().Err()
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		b.Start(ctx)
	}()
	waitForUpdateTest(t, started, "handler to start")
	cancel()
	select {
	case <-stopped:
		t.Fatal("Start returned while a dispatched handler was running")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	waitForUpdateTest(t, stopped, "Start to return after the handler")
}

// An update whose preparation exits counts as dispatched, so polling goes on
// with the remaining worker instead of replaying it.
func TestBot_StartConfirmsUpdateWhosePreparationExits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	offsets := make(chan string, 4)
	var calls int32
	b, err := New("xxx", WithSkipGetMe(), WithWorkers(2), WithErrorsHandler(func(error) {}),
		WithDefaultHandler(func(context.Context, *Bot, *models.Update) {}),
		WithHTTPClient(time.Second, getUpdatesClientFunc(func(req *http.Request) (*http.Response, error) {
			offsets <- req.FormValue("offset")
			if atomic.AddInt32(&calls, 1) == 1 {
				return getUpdatesJSONResponse(`{"ok":true,"result":[{"update_id":7},{"update_id":8}]}`), nil
			}
			<-req.Context().Done()
			return nil, req.Context().Err()
		})))
	if err != nil {
		t.Fatal(err)
	}
	b.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		if update.ID == 8 {
			runtime.Goexit()
		}
		return false
	}, b.defaultHandlerFunc)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		b.Start(ctx)
	}()
	for _, want := range []string{"1", "9"} {
		if got := waitForUpdateTest(t, offsets, "poll"); got != want {
			t.Errorf("poll offset = %q, want %q", got, want)
		}
	}
	cancel()
	waitForUpdateTest(t, stopped, "polling to stop")
}

// When the only worker exits during preparation the run stops, and the next
// run resumes after that update instead of replaying it forever.
func TestBot_StartResumesAfterUpdateWhosePreparationExits(t *testing.T) {
	b, err := New("xxx", WithSkipGetMe(), WithWorkers(1), WithNotAsyncHandlers(), WithErrorsHandler(func(error) {}),
		WithDefaultHandler(func(context.Context, *Bot, *models.Update) {}),
		WithHTTPClient(time.Second, getUpdatesClientFunc(func(req *http.Request) (*http.Response, error) {
			if req.FormValue("offset") == "1" {
				return getUpdatesJSONResponse(`{"ok":true,"result":[{"update_id":7},{"update_id":8},{"update_id":9}]}`), nil
			}
			<-req.Context().Done()
			return nil, req.Context().Err()
		})))
	if err != nil {
		t.Fatal(err)
	}
	b.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		if update.ID == 8 {
			runtime.Goexit()
		}
		return false
	}, b.defaultHandlerFunc)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		b.Start(context.Background())
	}()
	waitForUpdateTest(t, stopped, "polling to stop after its only worker exits")
	if got := atomic.LoadInt64(&b.nextUpdateOffset); got != 9 {
		t.Errorf("next update offset = %d, want 9", got)
	}
}

// A negative offset makes Telegram drop older updates. Polling must resume
// from the batch it got, so an update not dispatched before shutdown is
// delivered after a restart.
func TestBot_StartWithNegativeOffsetKeepsUndispatchedUpdate(t *testing.T) {
	var mx sync.Mutex
	pending := []int64{100, 101}
	var handled []int64
	firstStarted := make(chan struct{})
	restarted := false
	b, err := New("xxx", WithSkipGetMe(), WithNotAsyncHandlers(), WithInitialOffset(-2), WithErrorsHandler(func(error) {}),
		WithDefaultHandler(func(ctx context.Context, _ *Bot, update *models.Update) {
			mx.Lock()
			handled = append(handled, update.ID)
			first := !restarted && update.ID == 100
			mx.Unlock()
			if first {
				close(firstStarted)
				<-ctx.Done()
			}
		}),
		WithHTTPClient(time.Second, getUpdatesClientFunc(func(req *http.Request) (*http.Response, error) {
			offset, _ := strconv.ParseInt(req.FormValue("offset"), 10, 64)
			mx.Lock()
			// Follow the getUpdates offset rules of the Bot API.
			if offset < 0 && len(pending) > int(-offset) {
				pending = pending[len(pending)-int(-offset):]
			}
			var result []string
			kept := pending[:0]
			for _, id := range pending {
				if offset <= 0 || id >= offset {
					kept = append(kept, id)
					result = append(result, fmt.Sprintf(`{"update_id":%d}`, id))
				}
			}
			pending = kept
			mx.Unlock()
			if len(result) == 0 {
				<-req.Context().Done()
				return nil, req.Context().Err()
			}
			return getUpdatesJSONResponse(`{"ok":true,"result":[` + strings.Join(result, ",") + `]}`), nil
		})))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		b.Start(ctx)
	}()
	waitForUpdateTest(t, firstStarted, "first handler to start")
	cancel()
	waitForUpdateTest(t, stopped, "polling to stop")

	mx.Lock()
	restarted = true
	pending = append(pending, 102, 103)
	mx.Unlock()
	resumeCtx, cancelResume := context.WithCancel(context.Background())
	defer cancelResume()
	resumed := make(chan struct{})
	go func() {
		defer close(resumed)
		b.Start(resumeCtx)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		mx.Lock()
		n := len(handled)
		mx.Unlock()
		if n >= 4 || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	cancelResume()
	waitForUpdateTest(t, resumed, "resumed polling to stop")
	mx.Lock()
	defer mx.Unlock()
	if want := []int64{100, 101, 102, 103}; !reflect.DeepEqual(handled, want) {
		t.Errorf("handled updates = %v, want %v", handled, want)
	}
}

// A batch fetched with a negative offset moves the cursor to the batch before
// any update is dispatched.
func Test_getUpdates_negativeOffsetResumesFromBatch(t *testing.T) {
	requested := make(chan struct{})
	b, err := New("xxx", WithSkipGetMe(), WithInitialOffset(-2),
		WithHTTPClient(time.Second, getUpdatesClientFunc(func(req *http.Request) (*http.Response, error) {
			if req.FormValue("offset") != "-2" {
				<-req.Context().Done()
				return nil, req.Context().Err()
			}
			close(requested)
			return getUpdatesJSONResponse(`{"ok":true,"result":[{"update_id":100},{"update_id":101}]}`), nil
		})))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		// Nobody reads the queue, so no update is dispatched.
		b.getUpdates(ctx, make(chan queuedUpdate))
	}()
	waitForUpdateTest(t, requested, "poll with the negative offset")
	cancel()
	waitForUpdateTest(t, stopped, "polling to stop")
	if got := atomic.LoadInt64(&b.nextUpdateOffset); got != 100 {
		t.Errorf("next update offset = %d, want 100", got)
	}
}

func TestBot_StartWithoutWorkersReportsError(t *testing.T) {
	reported := make(chan error, 2)
	b, err := New("xxx", WithSkipGetMe(), WithWorkers(0),
		WithErrorsHandler(func(err error) { reported <- err }))
	if err != nil {
		t.Fatal(err)
	}

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		b.Start(context.Background())
	}()
	waitForUpdateTest(t, stopped, "Start without workers to return")
	if got := waitForUpdateTest(t, reported, "workers error"); !strings.Contains(got.Error(), "without update workers") {
		t.Errorf("reported %v, want the missing workers error", got)
	}
}

// Cancellation while a matcher runs must not leave the update unconfirmed:
// its handler still runs, so replaying it would run it twice.
func TestBot_StartConfirmsUpdateCanceledDuringPreparation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	matching := make(chan struct{})
	release := make(chan struct{})
	var handled int32
	b, err := New("xxx", WithSkipGetMe(), WithWorkers(1), WithErrorsHandler(func(error) {}),
		WithDefaultHandler(func(context.Context, *Bot, *models.Update) { atomic.AddInt32(&handled, 1) }),
		WithHTTPClient(time.Second, getUpdatesClientFunc(func(req *http.Request) (*http.Response, error) {
			if req.FormValue("offset") == "1" {
				return getUpdatesJSONResponse(`{"ok":true,"result":[{"update_id":7}]}`), nil
			}
			<-req.Context().Done()
			return nil, req.Context().Err()
		})))
	if err != nil {
		t.Fatal(err)
	}
	b.RegisterHandlerMatchFunc(func(*models.Update) bool {
		close(matching)
		<-release
		return false
	}, b.defaultHandlerFunc)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		b.Start(ctx)
	}()
	waitForUpdateTest(t, matching, "matcher to run")
	cancel()
	close(release)
	waitForUpdateTest(t, stopped, "polling to stop")
	if got := atomic.LoadInt32(&handled); got != 1 {
		t.Errorf("handled %d times, want 1", got)
	}
	if got := atomic.LoadInt64(&b.nextUpdateOffset); got != 8 {
		t.Errorf("next update offset = %d, want 8", got)
	}
}

// Cancellation while the poller waits to queue the next update still confirms
// the updates already dispatched.
func TestBot_StartConfirmsDispatchedUpdatesWhenCanceledWhileQueuing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	firstStarted := make(chan struct{})
	b, err := New("xxx", WithSkipGetMe(), WithWorkers(1), WithNotAsyncHandlers(), WithUpdatesChannelCap(0),
		WithDefaultHandler(func(ctx context.Context, _ *Bot, update *models.Update) {
			if update.ID == 1 {
				close(firstStarted)
				<-ctx.Done()
			}
		}),
		WithHTTPClient(time.Second, getUpdatesClientFunc(func(req *http.Request) (*http.Response, error) {
			if req.FormValue("offset") == "1" {
				return getUpdatesJSONResponse(`{"ok":true,"result":[{"update_id":1},{"update_id":2}]}`), nil
			}
			<-req.Context().Done()
			return nil, req.Context().Err()
		})))
	if err != nil {
		t.Fatal(err)
	}
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		b.Start(ctx)
	}()
	waitForUpdateTest(t, firstStarted, "first handler to start")
	cancel()
	waitForUpdateTest(t, stopped, "polling to stop")
	if got := atomic.LoadInt64(&b.nextUpdateOffset); got != 2 {
		t.Errorf("next update offset = %d, want 2", got)
	}
}

func TestBot_InitialPollingOffset(t *testing.T) {
	for _, tt := range []struct {
		name        string
		offset      int64
		wantInitial string
		wantAfter   string
	}{
		{name: "positive", offset: 100, wantInitial: "100", wantAfter: "100"},
		{name: "zero", offset: 0, wantInitial: "0", wantAfter: "0"},
		{name: "negative", offset: -1, wantInitial: "-1", wantAfter: "0"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var offsets []string
			b, err := New("xxx", WithSkipGetMe(), WithInitialOffset(tt.offset),
				WithHTTPClient(time.Second, getUpdatesClientFunc(func(req *http.Request) (*http.Response, error) {
					offsets = append(offsets, req.FormValue("offset"))
					if len(offsets) == 1 {
						return getUpdatesJSONResponse(`{"ok":true,"result":[]}`), nil
					}
					cancel()
					return nil, req.Context().Err()
				})),
			)
			if err != nil {
				t.Fatal(err)
			}
			b.Start(ctx)

			if len(offsets) != 2 || offsets[0] != tt.wantInitial || offsets[1] != tt.wantAfter {
				t.Errorf("poll offsets = %q, want [%q %q]", offsets, tt.wantInitial, tt.wantAfter)
			}
		})
	}
}

func TestBot_NegativeOffsetAfterPollFailure(t *testing.T) {
	for _, tt := range []struct {
		name          string
		conflict      string
		offsetApplied bool
		wantRetry     string
		wantDelivered []int64
	}{
		{
			name:          "truncated response",
			offsetApplied: true,
			wantRetry:     "0",
			wantDelivered: []int64{42, 43},
		},
		{
			name:          "webhook active",
			conflict:      "Conflict: can't use getUpdates method while webhook is active; use deleteWebhook to delete the webhook first",
			wantRetry:     "-1",
			wantDelivered: []int64{43},
		},
		{
			name:          "long poll interrupted",
			conflict:      "Conflict: terminated by setWebhook request",
			offsetApplied: true,
			wantRetry:     "0",
			wantDelivered: []int64{42, 43},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			testNegativeOffsetAfterPollFailure(t, tt.conflict, tt.offsetApplied, tt.wantRetry, tt.wantDelivered)
		})
	}
}

func testNegativeOffsetAfterPollFailure(t *testing.T, conflict string, offsetApplied bool, wantRetry string, wantDelivered []int64) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pending := []models.Update{{ID: 40}, {ID: 42}}
	var offsets []string
	var delivered []int64
	var pollErrors []error
	b, err := New("xxx", WithSkipGetMe(), WithInitialOffset(-1), WithNotAsyncHandlers(),
		WithErrorsHandler(func(err error) { pollErrors = append(pollErrors, err) }),
		WithDefaultHandler(func(_ context.Context, _ *Bot, update *models.Update) {
			delivered = append(delivered, update.ID)
		}),
		WithHTTPClient(time.Second, getUpdatesClientFunc(func(req *http.Request) (*http.Response, error) {
			offset := req.FormValue("offset")
			offsets = append(offsets, offset)
			if len(offsets) == 3 {
				cancel()
				return nil, req.Context().Err()
			}
			isFirstPoll := len(offsets) == 1
			if offset == "-1" && (!isFirstPoll || offsetApplied) {
				pending = pending[len(pending)-1:]
			}
			if isFirstPoll {
				pending = append(pending, models.Update{ID: 43})
				if conflict != "" {
					resp := getUpdatesJSONResponse(fmt.Sprintf(`{"ok":false,"error_code":409,"description":%q}`, conflict))
					resp.StatusCode = http.StatusConflict
					return resp, nil
				}
				return getUpdatesJSONResponse(`{"ok":true,"result":[{"update_id":42}`), nil
			}
			result, err := json.Marshal(pending)
			if err != nil {
				t.Fatal(err)
			}
			return getUpdatesJSONResponse(`{"ok":true,"result":` + string(result) + `}`), nil
		})),
	)
	if err != nil {
		t.Fatal(err)
	}
	b.Start(ctx)

	if want := []string{"-1", wantRetry, "44"}; !reflect.DeepEqual(offsets, want) {
		t.Errorf("poll offsets = %q, want %q", offsets, want)
	}
	if !reflect.DeepEqual(delivered, wantDelivered) {
		t.Errorf("delivered updates = %v, want %v", delivered, wantDelivered)
	}
	if len(pollErrors) != 1 {
		t.Fatalf("poll errors = %v, want one", pollErrors)
	}
	if conflict != "" && (!errors.Is(pollErrors[0], ErrorConflict) || !strings.Contains(pollErrors[0].Error(), conflict)) {
		t.Errorf("poll error %v must wrap ErrorConflict and include the server description %q", pollErrors[0], conflict)
	}
}

func TestBot_NegativeOffsetAfterPollingRestart(t *testing.T) {
	for _, tt := range []struct {
		name       string
		firstReply func() (*http.Response, error)
		wantError  error
		want       []string
	}{
		{
			name: "canceled during transport",
			firstReply: func() (*http.Response, error) {
				return nil, context.Canceled
			},
			want: []string{"-1", "0"},
		},
		{
			name: "Telegram rate limit",
			firstReply: func() (*http.Response, error) {
				resp := getUpdatesJSONResponse(`{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 1","parameters":{"retry_after":1}}`)
				resp.StatusCode = http.StatusTooManyRequests
				return resp, nil
			},
			want: []string{"-1", "-1"},
		},
		{
			name: "Telegram rejected token",
			firstReply: func() (*http.Response, error) {
				resp := getUpdatesJSONResponse(`{"ok":false,"error_code":401,"description":"Unauthorized: invalid token specified"}`)
				resp.StatusCode = http.StatusUnauthorized
				return resp, nil
			},
			wantError: ErrorUnauthorized,
			want:      []string{"-1", "-1"},
		},
		{
			name: "custom transport returns rate limit type",
			firstReply: func() (*http.Response, error) {
				return nil, &TooManyRequestsError{RetryAfter: 1}
			},
			want: []string{"-1", "0"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			firstCtx, cancelFirst := context.WithCancel(context.Background())
			defer cancelFirst()
			restartCtx, cancelRestart := context.WithCancel(context.Background())
			defer cancelRestart()

			var offsets []string
			var restarting bool
			b, err := New("xxx", WithSkipGetMe(), WithInitialOffset(-1),
				WithErrorsHandler(func(err error) {
					if tt.wantError != nil && !errors.Is(err, tt.wantError) {
						t.Errorf("poll error = %v, want %v", err, tt.wantError)
					}
					cancelFirst()
				}),
				WithHTTPClient(time.Second, getUpdatesClientFunc(func(req *http.Request) (*http.Response, error) {
					offsets = append(offsets, req.FormValue("offset"))
					if restarting {
						cancelRestart()
						return nil, restartCtx.Err()
					}
					resp, err := tt.firstReply()
					if errors.Is(err, context.Canceled) {
						cancelFirst()
					}
					return resp, err
				})),
			)
			if err != nil {
				t.Fatal(err)
			}
			b.Start(firstCtx)
			restarting = true
			b.Start(restartCtx)

			if !reflect.DeepEqual(offsets, tt.want) {
				t.Errorf("poll offsets = %q, want %q", offsets, tt.want)
			}
		})
	}
}

// A request canceled before it is sent must not run beforeDispatch, which
// gives up a negative offset as if the request had reached Telegram.
func TestBot_RawRequestCanceledBeforeDispatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b, err := New("xxx", WithSkipGetMe(), WithHTTPClient(time.Second, getUpdatesClientFunc(func(*http.Request) (*http.Response, error) {
		t.Error("request dispatched despite cancellation")
		return nil, errors.New("unexpected request")
	})))
	if err != nil {
		t.Fatal(err)
	}
	dispatched := false
	var updates []json.RawMessage
	errRequest := b.rawRequestWithToken(ctx, b.Token(), "getUpdates", &getUpdatesParams{Offset: -1}, &updates, func() { dispatched = true })
	if !errors.Is(errRequest, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", errRequest)
	}
	if dispatched {
		t.Error("beforeDispatch ran for a canceled request")
	}
}

func TestBot_NegativeOffsetAfterRequestPreparationFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	restartCtx, cancelRestart := context.WithCancel(context.Background())
	defer cancelRestart()

	var offsets []string
	var prepareErr error
	var restarting bool
	b, err := New("123:invalid\ntoken", WithSkipGetMe(), WithInitialOffset(-1),
		WithErrorsHandler(func(err error) {
			prepareErr = err
			cancel()
		}),
		WithHTTPClient(time.Second, getUpdatesClientFunc(func(req *http.Request) (*http.Response, error) {
			offsets = append(offsets, req.FormValue("offset"))
			if !restarting {
				t.Fatal("request with malformed token reached the HTTP client")
			}
			cancelRestart()
			return nil, restartCtx.Err()
		})),
	)
	if err != nil {
		t.Fatal(err)
	}
	b.Start(ctx)
	if prepareErr == nil {
		t.Fatal("malformed request was not reported")
	}

	b.SetToken("123:valid")
	restarting = true
	b.Start(restartCtx)
	if !reflect.DeepEqual(offsets, []string{"-1"}) {
		t.Errorf("poll offsets = %q, want [-1]", offsets)
	}
}

func TestBot_NegativeOffsetAfterDialFailure(t *testing.T) {
	for _, tt := range []struct {
		name       string
		redirect   bool
		wantOffset string
	}{
		{name: "first dial refused", wantOffset: "-1"},
		{name: "redirected dial refused", redirect: true, wantOffset: "0"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			firstCtx, cancelFirst := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancelFirst()
			restartCtx, cancelRestart := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancelRestart()

			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			refusedURL := "http://" + listener.Addr().String()

			firstURL := refusedURL
			if tt.redirect {
				redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if got := r.FormValue("offset"); got != "-1" {
						t.Errorf("offset received before redirect = %q, want -1", got)
					}
					http.Redirect(w, r, refusedURL, http.StatusTemporaryRedirect)
				}))
				defer redirect.Close()
				firstURL = redirect.URL
			}

			resumedOffsets := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resumedOffsets <- r.FormValue("offset")
				w.WriteHeader(http.StatusNoContent)
				cancelRestart()
			}))
			defer server.Close()

			var pollErr error
			client := &http.Client{Transport: &http.Transport{}, Timeout: 5 * time.Second}
			defer client.CloseIdleConnections()
			b, err := New("xxx", WithSkipGetMe(), WithInitialOffset(-1), WithServerURL(firstURL),
				WithHTTPClient(time.Second, client),
				WithErrorsHandler(func(err error) {
					if firstCtx.Err() == nil {
						pollErr = err
						cancelFirst()
					}
				}),
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}
			b.Start(firstCtx)
			var dialErr *net.OpError
			if !errors.As(pollErr, &dialErr) || dialErr.Op != "dial" {
				t.Fatalf("poll error = %v, want dial failure", pollErr)
			}

			WithServerURL(server.URL)(b)
			b.Start(restartCtx)
			if got := waitForUpdateTest(t, resumedOffsets, "first resumed poll"); got != tt.wantOffset {
				t.Errorf("resumed offset = %q, want %q", got, tt.wantOffset)
			}
		})
	}
}
