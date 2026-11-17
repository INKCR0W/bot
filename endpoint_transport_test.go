package bot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBot_EndpointURLs(t *testing.T) {
	const token = "123:fake-token"
	const filePath = "photos/logo.jpg"
	for _, tc := range []struct {
		name        string
		basePath    string
		getMePath   string
		getFilePath string
		download    string
		testMode    bool
	}{
		{
			name:        "production with trailing slash",
			basePath:    "/",
			getMePath:   "/bot123:fake-token/getMe",
			getFilePath: "/bot123:fake-token/getFile",
			download:    "/file/bot123:fake-token/photos/logo.jpg",
		},
		{
			name:        "test environment",
			getMePath:   "/bot123:fake-token/test/getMe",
			getFilePath: "/bot123:fake-token/test/getFile",
			download:    "/file/bot123:fake-token/test/photos/logo.jpg",
			testMode:    true,
		},
		{
			name:        "test environment behind a path prefix",
			basePath:    "/gateway%2Fstaging///",
			getMePath:   "/gateway%2Fstaging/bot123:fake-token/test/getMe",
			getFilePath: "/gateway%2Fstaging/bot123:fake-token/test/getFile",
			download:    "/gateway%2Fstaging/file/bot123:fake-token/test/photos/logo.jpg",
			testMode:    true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				var body string
				switch req.Method + " " + req.RequestURI {
				case "POST " + tc.getMePath:
					body = `{"ok":true,"result":{"id":123,"is_bot":true,"first_name":"test","username":"test_bot"}}`
				case "POST " + tc.getFilePath:
					body = fmt.Sprintf(`{"ok":true,"result":{"file_id":"id","file_path":%q}}`, filePath)
				case "GET " + tc.download:
					body = "image contents"
				default:
					t.Errorf("unexpected request %s %s", req.Method, req.RequestURI)
					http.NotFound(w, req)
					return
				}
				if _, err := io.WriteString(w, body); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()

			options := []Option{WithServerURL(server.URL + tc.basePath)}
			if tc.testMode {
				options = append(options, UseTestEnvironment())
			}
			b, err := New(token, options...)
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}
			ctx := context.Background()
			file, err := b.GetFile(ctx, &GetFileParams{FileID: "id"})
			if err != nil {
				t.Fatalf("GetFile() error: %v", err)
			}
			link := b.FileDownloadLink(file)
			if want := server.URL + tc.download; link != want {
				t.Fatalf("FileDownloadLink() = %q, want %q", link, want)
			}
			resp, err := server.Client().Get(link)
			if err != nil {
				t.Fatalf("download error: %v", err)
			}
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil || resp.StatusCode != http.StatusOK || string(data) != "image contents" {
				t.Fatalf("download response = (%d, %q, %v), want (200, image contents, nil)", resp.StatusCode, data, err)
			}
		})
	}
}

func TestBot_SendMessageMigrationID(t *testing.T) {
	const chatID int64 = -1001234567890
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := fmt.Fprintf(w, `{"ok":false,"error_code":400,"description":"group migrated","parameters":{"migrate_to_chat_id":%d}}`, chatID); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	b, err := New("123:fake-token", WithServerURL(server.URL), WithSkipGetMe())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	_, err = b.SendMessage(context.Background(), &SendMessageParams{ChatID: -123, Text: "hello"})
	var migration *MigrateError
	if !errors.As(err, &migration) || !IsMigrateError(err) {
		t.Fatalf("SendMessage() error = %v, want *MigrateError", err)
	}
	if got := migration.MigrateToChatID; got != chatID {
		t.Fatalf("migration destination = %d, want %d", got, chatID)
	}
	if got, want := err.Error(), "bad request: group migrated: migrate_to_chat_id -1001234567890"; got != want {
		t.Errorf("error message = %q, want %q", got, want)
	}
}
