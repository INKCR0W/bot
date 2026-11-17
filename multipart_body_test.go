package bot

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
)

const spooledTestUploadBytes = 2 << 20

func TestRawRequestReplaysSpooledMultipart(t *testing.T) {
	tempDir := useUploadTempDir(t)
	digests := make(chan [sha256.Size]byte, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assertActiveSpoolFileCount(t, tempDir)
		digest := sha256.New()
		length, err := io.Copy(digest, req.Body)
		if err != nil || length != req.ContentLength || length <= spooledTestUploadBytes {
			t.Errorf("read multipart body: size=%d, Content-Length=%d, error=%v", length, req.ContentLength, err)
		}
		var sum [sha256.Size]byte
		copy(sum[:], digest.Sum(nil))
		select {
		case digests <- sum:
		default:
			t.Error("unexpected extra request")
		}
		if req.URL.Path != "/retry" {
			http.Redirect(w, req, "/retry", http.StatusTemporaryRedirect)
			return
		}
		if _, err := io.WriteString(w, `{"ok":true,"result":{"message_id":42}}`); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	b, err := New("123:test", WithSkipGetMe(), WithServerURL(server.URL), WithHTTPClient(5*time.Second, server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	msg, err := b.SendDocument(ctx, uploadTestDocument(io.LimitReader(uploadZeros{}, spooledTestUploadBytes)))
	if err != nil {
		t.Fatal(err)
	}
	if msg.ID != 42 {
		t.Errorf("message ID = %d, want 42", msg.ID)
	}
	if len(digests) != 2 {
		t.Fatalf("received %d requests, want 2", len(digests))
	}
	if original, replay := <-digests, <-digests; original != replay {
		t.Error("replayed multipart body differs from the original")
	}
}

func TestRawRequestRemovesSpooledMultipartOnFailure(t *testing.T) {
	t.Run("source read fails", func(t *testing.T) {
		tempDir := useUploadTempDir(t)
		readErr := errors.New("upload source failed")
		source := io.MultiReader(io.LimitReader(uploadZeros{}, spooledTestUploadBytes), uploadReadFunc(func([]byte) (int, error) {
			assertActiveSpoolFileCount(t, tempDir)
			return 0, readErr
		}))
		client := unexpectedUploadClient(t)
		b, err := New("123:test", WithSkipGetMe(), WithHTTPClient(time.Second, client))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := b.SendDocument(context.Background(), uploadTestDocument(source)); !errors.Is(err, readErr) {
			t.Fatalf("SendDocument error = %v, want %v", err, readErr)
		}
	})
	t.Run("canceled during upload preparation", func(t *testing.T) {
		tempDir := useUploadTempDir(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		source := io.MultiReader(io.LimitReader(uploadZeros{}, spooledTestUploadBytes), uploadReadFunc(func(p []byte) (int, error) {
			assertActiveSpoolFileCount(t, tempDir)
			cancel()
			p[0] = 0
			return 1, nil
		}))
		client := unexpectedUploadClient(t)
		b, err := New("123:test", WithSkipGetMe(), WithHTTPClient(time.Second, client))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := b.SendDocument(ctx, uploadTestDocument(source)); !errors.Is(err, context.Canceled) {
			t.Fatalf("SendDocument error = %v, want context.Canceled", err)
		}
	})
	t.Run("transport fails", func(t *testing.T) {
		tempDir := useUploadTempDir(t)
		transportErr := errors.New("transport failed")
		client := uploadClientFunc(func(req *http.Request) (*http.Response, error) {
			assertActiveSpoolFileCount(t, tempDir)
			if err := req.Body.Close(); err != nil {
				t.Error(err)
			}
			return nil, transportErr
		})
		b, err := New("123:test", WithSkipGetMe(), WithHTTPClient(time.Second, client))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := b.SendDocument(context.Background(), uploadTestDocument(io.LimitReader(uploadZeros{}, spooledTestUploadBytes))); !errors.Is(err, transportErr) {
			t.Fatalf("SendDocument error = %v, want %v", err, transportErr)
		}
	})
}

func TestRawRequestKeepsSpoolUntilAsyncTransportClosesBody(t *testing.T) {
	t.Run("first attempt", func(t *testing.T) {
		testAsyncMultipartTransport(t, false)
	})
	t.Run("redirect replay", func(t *testing.T) {
		testAsyncMultipartTransport(t, true)
	})
}

func testAsyncMultipartTransport(t *testing.T, redirect bool) {
	t.Helper()
	useUploadTempDir(t)
	readAllowed := make(chan struct{})
	finished := make(chan error, 1)
	client := &http.Client{Transport: uploadRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if redirect && req.URL.Path != "/retry" {
			if err := req.Body.Close(); err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusTemporaryRedirect,
				Header:     http.Header{"Location": {"/retry"}},
				Body:       http.NoBody,
			}, nil
		}
		go func(body io.ReadCloser, length int64) {
			<-readAllowed
			read, err := io.Copy(io.Discard, body)
			closeErr := body.Close()
			if err != nil || read != length || closeErr != nil {
				finished <- fmt.Errorf("read %d of %d bytes, read error=%v, close error=%v", read, length, err, closeErr)
				return
			}
			finished <- nil
		}(req.Body, req.ContentLength)
		return (&clientMock{}).Do(req)
	})}
	if redirect {
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return nil }
	}
	b, err := New("123:test", WithSkipGetMe(), WithHTTPClient(time.Second, client))
	if err != nil {
		t.Fatal(err)
	}
	err = b.rawRequest(context.Background(), "sendDocument", uploadTestDocument(io.LimitReader(uploadZeros{}, spooledTestUploadBytes)), nil)
	close(readAllowed)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("transport did not finish reading the request")
	}
}

func TestRawRequestClosesRetainedRejectedReplay(t *testing.T) {
	redirectErr := errors.New("redirect denied")
	for _, tc := range []struct {
		name                  string
		decision              error
		wantErr               error
		wantReplacementClosed bool
	}{
		{name: "return last response", decision: http.ErrUseLastResponse},
		{name: "return redirect error", decision: redirectErr, wantErr: redirectErr, wantReplacementClosed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useUploadTempDir(t)
			var proposed *http.Request
			var replay io.ReadCloser
			replacement := &uploadReplacementBody{Reader: strings.NewReader("replacement")}
			client := &http.Client{
				CheckRedirect: func(req *http.Request, _ []*http.Request) error {
					proposed, replay = req, req.Body
					req.Body = replacement
					return tc.decision
				},
				Transport: uploadRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
					if req.URL.Path == "/retry" {
						t.Error("client followed a rejected redirect")
					}
					if err := req.Body.Close(); err != nil {
						return nil, err
					}
					return &http.Response{
						StatusCode: http.StatusTemporaryRedirect,
						Header:     http.Header{"Location": {"/retry"}},
						Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":42}}`)),
					}, nil
				}),
			}
			b, err := New("123:test", WithSkipGetMe(), WithHTTPClient(time.Second, client))
			if err != nil {
				t.Fatal(err)
			}
			msg, err := b.SendDocument(context.Background(), uploadTestDocument(io.LimitReader(uploadZeros{}, spooledTestUploadBytes)))
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("SendDocument error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr == nil && (msg == nil || msg.ID != 42) {
				t.Errorf("expected last response with message ID 42, got %+v", msg)
			}
			if proposed == nil || replay == nil {
				t.Fatal("redirect callback did not receive the proposed replay")
			}
			if proposed.Body != replacement {
				t.Error("client changed the callback's replacement body")
			}
			if replacement.closed != tc.wantReplacementClosed {
				t.Errorf("callback's replacement body closed = %t, want %t", replacement.closed, tc.wantReplacementClosed)
			}
			var probe [1]byte
			if _, err := replay.Read(probe[:]); !errors.Is(err, os.ErrClosed) {
				t.Errorf("retained replay was not closed, read error = %v", err)
			}
			if err := replay.Close(); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestRawRequestReleasesUnreadReplayOnCancellation(t *testing.T) {
	t.Run("canceled context", func(t *testing.T) {
		testCanceledUploadReplay(t, 0)
	})
	t.Run("client timeout", func(t *testing.T) {
		testCanceledUploadReplay(t, 20*time.Millisecond)
	})
}

func testCanceledUploadReplay(t *testing.T, clientTimeout time.Duration) {
	t.Helper()
	useUploadTempDir(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var unread, started io.ReadCloser
	defer func() {
		for _, body := range []io.ReadCloser{started, unread} {
			if body != nil {
				if err := body.Close(); err != nil {
					t.Error(err)
				}
			}
		}
	}()
	client := &http.Client{Timeout: clientTimeout, Transport: uploadRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if err := req.Body.Close(); err != nil {
			return nil, err
		}
		var err error
		started, err = req.GetBody()
		if err != nil {
			return nil, err
		}
		var probe [1]byte
		if _, err := io.ReadFull(started, probe[:]); err != nil {
			return nil, err
		}
		unread, err = req.GetBody()
		if err != nil {
			return nil, err
		}
		if clientTimeout == 0 {
			cancel()
		}
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}
	b, err := New("123:test", WithSkipGetMe(), WithHTTPClient(time.Second, client))
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.SendDocument(ctx, uploadTestDocument(io.LimitReader(uploadZeros{}, spooledTestUploadBytes)))
	if clientTimeout > 0 {
		var timeoutErr net.Error
		if !errors.As(err, &timeoutErr) || !timeoutErr.Timeout() || ctx.Err() != nil {
			t.Fatalf("expected HTTP client timeout with an active caller context, got %v, context error=%v", err, ctx.Err())
		}
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("SendDocument error = %v, want context.Canceled", err)
	}
	if unread == nil || started == nil {
		t.Fatal("transport did not create both replay bodies")
	}
	var probe [1]byte
	if _, err := unread.Read(probe[:]); !errors.Is(err, os.ErrClosed) {
		t.Errorf("abandoned replay read error = %v, want os.ErrClosed", err)
	}
	if size, err := io.Copy(io.Discard, started); err != nil || size < spooledTestUploadBytes {
		t.Errorf("started replay read %d bytes, error = %v", size, err)
	}
	if err := started.Close(); err != nil {
		t.Fatal(err)
	}
	file := unread.(*multipartFileReader).body.file
	if _, err := file.ReadAt(probe[:], 0); !errors.Is(err, os.ErrClosed) {
		t.Errorf("abandoned replay kept the file open, read error = %v", err)
	}
}

func TestRawRequestKeepsLargeMultipartInMemoryOnJS(t *testing.T) {
	if runtime.GOOS != "js" {
		t.Skip("requires Go's JavaScript runtime")
	}
	client := uploadClientFunc(func(req *http.Request) (*http.Response, error) {
		length, err := io.Copy(io.Discard, req.Body)
		if closeErr := req.Body.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		if err != nil || length != req.ContentLength || length <= spooledTestUploadBytes {
			t.Errorf("read multipart body: size=%d, Content-Length=%d, error=%v", length, req.ContentLength, err)
		}
		return (&clientMock{}).Do(req)
	})
	b, err := New("123:test", WithSkipGetMe(), WithHTTPClient(time.Second, client))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.rawRequest(context.Background(), "sendDocument", uploadTestDocument(io.LimitReader(uploadZeros{}, spooledTestUploadBytes)), nil); err != nil {
		t.Fatal(err)
	}
}

func TestMultipartSpoolEventuallyClosesWithUnusedReplay(t *testing.T) {
	useUploadTempDir(t)
	var body multipartBody
	if _, err := io.Copy(&body, io.LimitReader(uploadZeros{}, spooledTestUploadBytes)); err != nil {
		t.Fatal(err)
	}
	if body.file == nil {
		t.Fatal("body never spooled")
	}
	if _, err := body.replayReader(); err != nil {
		t.Fatal(err)
	}
	// net/http may discard a replay without closing it when the redirect is rejected.
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	var probe [1]byte
	for {
		runtime.GC()
		if _, err := body.file.ReadAt(probe[:], 0); errors.Is(err, os.ErrClosed) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("unused replay kept spool open after GC")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestMultipartSpoolDoesNotSurviveProcessExit(t *testing.T) {
	const childEnv = "TELEGRAM_BOT_TEST_SPOOL_CHILD"
	if os.Getenv(childEnv) == "1" {
		var body multipartBody
		if _, err := io.Copy(&body, io.LimitReader(uploadZeros{}, spooledTestUploadBytes)); err != nil {
			t.Fatal(err)
		}
		if body.file == nil {
			t.Fatal("body never spooled")
		}
		os.Exit(0)
	}

	useUploadTempDir(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMultipartSpoolDoesNotSurviveProcessExit$")
	cmd.Env = append(os.Environ(), childEnv+"=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("spooling subprocess: %v, output=%s", err, output)
	}
}

func TestRawRequestKeepsSmallMultipartInMemory(t *testing.T) {
	tempDir := useUploadTempDir(t)
	client := uploadClientFunc(func(req *http.Request) (*http.Response, error) {
		assertTempFileCount(t, tempDir, 0)
		if _, err := io.Copy(io.Discard, req.Body); err != nil {
			return nil, err
		}
		return (&clientMock{}).Do(req)
	})
	b, err := New("123:test", WithSkipGetMe(), WithHTTPClient(time.Second, client))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.rawRequest(context.Background(), "sendDocument", uploadTestDocument(io.LimitReader(uploadZeros{}, 512)), nil); err != nil {
		t.Fatal(err)
	}
	assertTempFileCount(t, tempDir, 0)
}

func TestRawRequestCopiesUploadInChunksWithoutCancellation(t *testing.T) {
	const contents = "document contents"
	source := uploadWithWriterTo{Reader: strings.NewReader(contents)}
	client := &clientMock{}
	b, err := New("123:test", WithSkipGetMe(), WithHTTPClient(time.Second, client))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.rawRequest(context.Background(), "sendDocument", uploadTestDocument(source), nil); err != nil {
		t.Fatal(err)
	}
	form, err := client.gotReq.MultipartReader()
	if err != nil {
		t.Fatal(err)
	}
	formData, err := form.ReadForm(1024)
	if err != nil {
		t.Fatal(err)
	}
	parts := formData.File["document"]
	if len(parts) != 1 || parts[0].Filename != "file.bin" {
		t.Fatalf("unexpected document parts: %+v", parts)
	}
	file, err := parts[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(file)
	if closeErr := file.Close(); closeErr != nil {
		t.Error(closeErr)
	}
	if err != nil || string(got) != contents {
		t.Fatalf("document content = %q, error=%v", got, err)
	}
}

func useUploadTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)
	t.Cleanup(func() {
		// Pending deletes on Windows can prevent a seemingly empty directory from
		// being removed. A nonrecursive removal verifies the spool is gone.
		deadline := time.NewTimer(5 * time.Second)
		defer deadline.Stop()
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			err := os.Remove(dir)
			if err == nil {
				return
			}
			select {
			case <-deadline.C:
				t.Errorf("upload temp directory could not be removed: %v", err)
				return
			case <-ticker.C:
			}
		}
	})
	return dir
}

func assertTempFileCount(t *testing.T, dir string, want int) {
	t.Helper()
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != want {
		t.Errorf("temporary files = %d, want %d (read error: %v)", len(files), want, err)
	}
}

func assertActiveSpoolFileCount(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		assertTempFileCount(t, dir, 1)
		return
	}
	assertTempFileCount(t, dir, 0)
}

func uploadTestDocument(source io.Reader) *SendDocumentParams {
	return &SendDocumentParams{ChatID: 1, Document: &models.InputFileUpload{Filename: "file.bin", Data: source}}
}

type uploadZeros struct{}

func (uploadZeros) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

type uploadClientFunc func(*http.Request) (*http.Response, error)

func (f uploadClientFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

type uploadRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f uploadRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type uploadReplacementBody struct {
	io.Reader
	closed bool
}

func (r *uploadReplacementBody) Close() error {
	r.closed = true
	return nil
}

type uploadWithWriterTo struct{ io.Reader }

func (uploadWithWriterTo) WriteTo(io.Writer) (int64, error) {
	return 0, errors.New("upload called WriterTo")
}

func unexpectedUploadClient(t *testing.T) uploadClientFunc {
	t.Helper()
	return func(*http.Request) (*http.Response, error) {
		t.Fatal("transport invoked after upload preparation failed")
		return nil, nil
	}
}

func TestSpooledRequestClientClosesReplayRejectedByDefaultPolicy(t *testing.T) {
	useUploadTempDir(t)
	b, err := New("123:test", WithSkipGetMe(), WithHTTPClient(time.Second, &http.Client{}))
	if err != nil {
		t.Fatal(err)
	}
	client, ok := b.clientForSpooledRequest().(*http.Client)
	if !ok || client.CheckRedirect == nil {
		t.Fatal("spooled request client does not check redirects")
	}

	var body multipartBody
	defer func() {
		if err := body.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := body.Write(make([]byte, multipartMemoryLimitBytes+1)); err != nil {
		t.Fatal(err)
	}
	if body.file == nil {
		t.Fatal("body was not spooled")
	}

	allowed, err := body.replayReader()
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CheckRedirect(&http.Request{Body: allowed}, make([]*http.Request, 9)); err != nil {
		t.Errorf("ninth redirect rejected: %v", err)
	}
	if err := allowed.Close(); err != nil {
		t.Error(err)
	}

	rejected, err := body.replayReader()
	if err != nil {
		t.Fatal(err)
	}
	err = client.CheckRedirect(&http.Request{Body: rejected}, make([]*http.Request, 10))
	if err == nil || err.Error() != "stopped after 10 redirects" {
		t.Errorf("tenth redirect error = %v, want stopped after 10 redirects", err)
	}
	var probe [1]byte
	if _, err := rejected.Read(probe[:]); !errors.Is(err, os.ErrClosed) {
		t.Errorf("rejected replay was not closed, read error = %v", err)
	}
}
