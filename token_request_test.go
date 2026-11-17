package bot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
)

type tokenTestClient func(*http.Request) (*http.Response, error)

func (client tokenTestClient) Do(req *http.Request) (*http.Response, error) {
	return client(req)
}

func TestNew_RedactsTokenFromRequestCreationError(t *testing.T) {
	const token = "123:fake-test-token"
	_, err := New(token, WithServerURL("https://invalid host"))
	if err == nil {
		t.Fatal("New accepted an invalid server URL")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatal("request creation error contains the token")
	}
	var parseErr *url.Error
	if !errors.As(err, &parseErr) {
		t.Fatalf("error %v lost its URL parse cause", err)
	}
	if want := "https://invalid host/bot***/getMe"; parseErr.URL != want {
		t.Fatalf("redacted URL = %q, want %q", parseErr.URL, want)
	}
}

func TestRequest_RedactsEncodedTokenFromRedirectError(t *testing.T) {
	const token = "123:fake-test-token"
	for _, colon := range []string{"%3A", "%3a"} {
		t.Run(colon, func(t *testing.T) {
			location := "/bot123" + colon + "fake-test-token/getWebhookInfo?label=123%3AFAKE-test-token"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, location, http.StatusTemporaryRedirect)
			}))
			defer server.Close()
			redirectErr := errors.New("redirect rejected")
			client := server.Client()
			client.CheckRedirect = func(*http.Request, []*http.Request) error { return redirectErr }
			b, err := New(token, WithSkipGetMe(), WithServerURL(server.URL), WithHTTPClient(time.Second, client))
			if err != nil {
				t.Fatal(err)
			}
			_, err = b.GetWebhookInfo(context.Background())
			var urlErr *url.Error
			if !errors.Is(err, redirectErr) || !errors.As(err, &urlErr) {
				t.Fatalf("error %v lost its redirect cause", err)
			}
			if strings.Contains(err.Error(), "123"+colon+"fake-test-token") {
				t.Fatal("redirect error contains the encoded token")
			}
			if want := "/bot***/getWebhookInfo?label=123%3AFAKE-test-token"; urlErr.URL != want {
				t.Fatalf("redacted redirect URL = %q, want %q", urlErr.URL, want)
			}
		})
	}
}

func TestRequest_RedactsTokenSnapshotAfterRotation(t *testing.T) {
	const token = "123:original-test-token"
	const nextToken = "123:replacement-test-token"
	const server = "https://example.test"
	const method = "getWebhookInfo"
	transportErr := &url.Error{Op: "Post", URL: server + "/bot" + token + "/" + method, Err: context.Canceled}
	var logs []string
	var b *Bot
	client := tokenTestClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != transportErr.URL {
			t.Fatalf("request URL = %q, want %q", req.URL, transportErr.URL)
		}
		if b.Token() != nextToken {
			t.Fatal("token was not rotated before sending")
		}
		return nil, fmt.Errorf("client failure: %w", transportErr)
	})
	var err error
	b, err = New(token, WithSkipGetMe(), WithServerURL(server), WithHTTPClient(time.Second, client), WithDebug(), WithDebugHandler(func(format string, args ...any) {
		logs = append(logs, fmt.Sprintf(format, args...))
		b.SetToken(nextToken)
	}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.GetWebhookInfo(context.Background())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v lost the cancellation cause", err)
	}
	if message := err.Error(); strings.Contains(message, token) || !strings.Contains(message, "client failure") {
		t.Fatal("request error must preserve the client context with the token redacted")
	}
	var redactedErr *url.Error
	if want := server + "/bot***/" + method; !errors.As(err, &redactedErr) || redactedErr.URL != want {
		t.Fatalf("URL error must expose the redacted URL %q", want)
	}
	if want := server + "/bot" + token + "/" + method; transportErr.URL != want {
		t.Fatal("request mutated the client's shared error")
	}
	if len(logs) != 1 || !strings.Contains(logs[0], server+"/bot***/"+method) || strings.Contains(logs[0], token) || strings.Contains(logs[0], nextToken) {
		t.Fatal("debug log must show the redacted request URL")
	}
}

func TestRequest_RedactionPreservesNetworkErrorClassification(t *testing.T) {
	const token = "123:fake-test-token"
	transportErr := &net.DNSError{Err: "lookup timed out", Name: token, IsTimeout: true, IsTemporary: true}
	urlErr := &url.Error{Op: "Post", URL: "https://example.test/bot" + token + "/getWebhookInfo", Err: transportErr}
	tests := []struct {
		name  string
		cause error
	}{
		{"direct transport error", transportErr},
		{"wrapped transport error", fmt.Errorf("client failure: %w", transportErr)},
		{"URL error", urlErr},
		{"wrapped URL error", fmt.Errorf("client failure: %w", urlErr)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := tokenTestClient(func(_ *http.Request) (*http.Response, error) {
				return nil, test.cause
			})
			b, err := New(token, WithSkipGetMe(), WithHTTPClient(time.Second, client))
			if err != nil {
				t.Fatal(err)
			}
			_, err = b.GetWebhookInfo(context.Background())
			var network net.Error
			if !errors.As(err, &network) {
				t.Fatalf("error %v does not expose a network error", err)
			}
			status := networkErrorStatus(network)
			if !status.Timeout() || !status.Temporary() {
				t.Fatalf("error %v lost its timeout or temporary classification", err)
			}
			if strings.Contains(err.Error(), token) || strings.Contains(network.Error(), token) {
				t.Fatal("request or network error contains the token")
			}
			var dns *net.DNSError
			if errors.As(err, &dns) {
				t.Fatal("request error chain exposes the original credential-bearing DNS error")
			}
		})
	}
}

func TestRequest_RetainsUnchangedTypedTransportErrors(t *testing.T) {
	const token = "123:fake-test-token"
	transportErr := &net.DNSError{Err: "lookup timed out", Name: "example.test", IsTimeout: true}
	urlErr := &url.Error{Op: "Post", URL: "https://example.test", Err: transportErr}
	client := tokenTestClient(func(_ *http.Request) (*http.Response, error) {
		return nil, urlErr
	})
	b, err := New(token, WithSkipGetMe(), WithHTTPClient(time.Second, client))
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.GetWebhookInfo(context.Background())
	var originalURL *url.Error
	var originalDNS *net.DNSError
	if !errors.As(err, &originalURL) || originalURL != urlErr || !errors.As(err, &originalDNS) || originalDNS != transportErr {
		t.Fatalf("error %v lost its unchanged URL or DNS error", err)
	}
}

func TestRequest_RetainsSafeCauseOfRedactedTransportError(t *testing.T) {
	const token = "123:fake-test-token"
	transportErr := &net.OpError{Op: "read", Net: "tcp", Err: context.DeadlineExceeded}
	client := tokenTestClient(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("request to %s failed: %w", req.URL, transportErr)
	})
	b, err := New(token, WithSkipGetMe(), WithHTTPClient(time.Second, client))
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.GetWebhookInfo(context.Background())
	var cause *net.OpError
	if !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &cause) || cause != transportErr {
		t.Fatalf("error %v lost its safe transport cause", err)
	}
	if strings.Contains(err.Error(), token) {
		t.Fatal("request error contains the token")
	}
}

type joinedTokenErrors []error

func (e joinedTokenErrors) Error() string { return e[0].Error() + "\n" + e[1].Error() }

func (e joinedTokenErrors) Unwrap() []error { return e }

func TestRequest_RetainsJoinedTransportCauses(t *testing.T) {
	const token = "123:fake-test-token"
	urlErr := &url.Error{Op: "Post", URL: "https://example.test/bot" + token + "/getWebhookInfo", Err: context.Canceled}
	dnsErr := &net.DNSError{Err: "lookup timed out", Name: "example.test", IsTimeout: true}
	err := redactRequestError(joinedTokenErrors{urlErr, dnsErr}, token)
	if strings.Contains(err.Error(), token) {
		t.Fatal("joined transport error contains the token")
	}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("redacted error %T lost its multi-error unwrap contract", err)
	}
	causes := joined.Unwrap()
	if len(causes) != 2 || causes[1] != dnsErr {
		t.Fatalf("redacted causes = %v, want a sanitized URL error followed by the original DNS error", causes)
	}
	var cleanURL *url.Error
	if !errors.As(causes[0], &cleanURL) || cleanURL.URL != "https://example.test/bot***/getWebhookInfo" || !errors.Is(cleanURL, context.Canceled) {
		t.Fatalf("redacted URL cause = %v", causes[0])
	}
}

type projectedTokenError struct {
	cause error
	url   *url.Error
}

func (e *projectedTokenError) Error() string { return "request failed" }

func (e *projectedTokenError) Unwrap() error { return e.cause }

func (e *projectedTokenError) As(target any) bool {
	if result, ok := target.(**url.Error); ok {
		*result = e.url
		return true
	}
	return false
}

func TestRequest_RedactsCustomURLProjection(t *testing.T) {
	const token = "123:fake-test-token"
	for _, name := range []string{"independent cause", "wraps its adapter"} {
		t.Run(name, func(t *testing.T) {
			original := &projectedTokenError{
				cause: context.Canceled,
				url:   &url.Error{Op: "Post", URL: "https://example.test/bot" + token + "/getWebhookInfo", Err: context.DeadlineExceeded},
			}
			var wantCause error = context.DeadlineExceeded
			if name == "wraps its adapter" {
				original.url.Err = fmt.Errorf("retry: %w", original)
				wantCause = context.Canceled
			}
			err := redactRequestError(original, token)
			var cleanURL *url.Error
			if !errors.As(err, &cleanURL) || cleanURL.URL != "https://example.test/bot***/getWebhookInfo" || !errors.Is(cleanURL, wantCause) {
				t.Fatalf("redacted projected URL error = %v", cleanURL)
			}
			if !errors.Is(err, context.Canceled) || errors.Unwrap(err) != context.Canceled {
				t.Fatal("redaction lost the adapter's ordinary cancellation cause")
			}
			var exposedAdapter *projectedTokenError
			if errors.As(err, &exposedAdapter) {
				t.Fatal("redaction exposes the original credential-bearing adapter")
			}
		})
	}
}

type selfProjectingTokenError string

func (e selfProjectingTokenError) Error() string { return "request failed" }

func (e selfProjectingTokenError) Unwrap() error { return context.Canceled }

func (e selfProjectingTokenError) As(target any) bool {
	if result, ok := target.(**url.Error); ok {
		*result = &url.Error{Op: "Post", URL: string(e), Err: e}
		return true
	}
	return false
}

func TestRequest_RedactsSelfProjectingValueError(t *testing.T) {
	const token = "123:fake-test-token"
	original := selfProjectingTokenError("https://example.test/bot" + token + "/getWebhookInfo")
	err := redactRequestError(original, token)
	var projected *url.Error
	if !errors.As(err, &projected) || projected.URL != "https://example.test/bot***/getWebhookInfo" {
		t.Fatalf("redacted projected URL error = %v", projected)
	}
	if !errors.Is(err, context.Canceled) || !errors.Is(projected, context.Canceled) {
		t.Fatal("redaction lost the adapter's ordinary cancellation cause")
	}
	var nested *url.Error
	var adapter selfProjectingTokenError
	if errors.As(projected.Err, &nested) || errors.As(projected.Err, &adapter) {
		t.Fatal("redacted projection exposes the original credential-bearing adapter")
	}
}

func TestSetToken_ConcurrentRequestsAndAccessors(t *testing.T) {
	const firstToken = "123:first-test-token"
	const secondToken = "456:second-test-token"
	const server = "https://example.test"
	client := tokenTestClient(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/bot" + firstToken + "/getWebhookInfo", "/bot" + secondToken + "/getWebhookInfo":
		default:
			return nil, fmt.Errorf("invalid request path %q", req.URL.Path)
		}
		return &http.Response{Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":{}}`))}, nil
	})
	b, err := New(firstToken, WithSkipGetMe(), WithServerURL(server), WithHTTPClient(time.Second, client))
	if err != nil {
		t.Fatal(err)
	}
	const iterations = 100
	var writers sync.WaitGroup
	writers.Add(1)
	start := make(chan struct{})
	go func() {
		defer writers.Done()
		<-start
		for i := 0; i < iterations; i++ {
			b.SetToken(firstToken)
			b.SetToken(secondToken)
		}
	}()
	defer writers.Wait()
	close(start)

	file := &models.File{FilePath: "documents/example.txt"}
	for i := 0; i < iterations; i++ {
		if token := b.Token(); token != firstToken && token != secondToken {
			t.Fatalf("unexpected token %q", token)
		}
		if id := b.ID(); id != 123 && id != 456 {
			t.Fatalf("unexpected bot ID %d", id)
		}
		link := b.FileDownloadLink(file)
		if link != server+"/file/bot"+firstToken+"/"+file.FilePath && link != server+"/file/bot"+secondToken+"/"+file.FilePath {
			t.Fatalf("unexpected file download link %q", link)
		}
		if _, err := b.GetWebhookInfo(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}
