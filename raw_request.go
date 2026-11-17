package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

type apiResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	Description string          `json:"description,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
	Parameters  struct {
		RetryAfter      int   `json:"retry_after,omitempty"`
		MigrateToChatID int64 `json:"migrate_to_chat_id,omitempty"`
	} `json:"parameters,omitempty"`
}

const webhookActiveDescription = "Conflict: can't use getUpdates method while webhook is active; use deleteWebhook to delete the webhook first"

var errWebhookActive = fmt.Errorf("%w, %s", ErrorConflict, webhookActiveDescription)

type unauthorizedPollError struct{ error }

func (e unauthorizedPollError) Unwrap() error { return e.error }

// rawRequest prepares a replayable multipart body for HTTP retries and redirects.
// Local telegram-bot-api servers reject empty multipart forms, so methods without
// any fields send no body or Content-Type.
func (b *Bot) rawRequest(ctx context.Context, method string, params any, dest any) error {
	return b.rawRequestWithToken(ctx, b.Token(), method, params, dest, nil)
}

func (b *Bot) rawRequestWithToken(ctx context.Context, token, method string, params, dest any, beforeDispatch func()) error {
	if ctx == nil {
		return fmt.Errorf("nil context for method %s", method)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	var requestBody multipartBody
	defer func() {
		if err := requestBody.Close(); err != nil {
			b.errorsHandler(fmt.Errorf("clean up request body for method %s: %w", method, err))
		}
	}()
	form := multipart.NewWriter(&requestBody)

	var fieldsCount int
	if params != nil && !reflect.ValueOf(params).IsNil() {
		requestForm := newRequestForm(form)
		requestForm.ctx = ctx
		var errFormData error
		fieldsCount, errFormData = requestForm.build(params)
		if errFormData != nil {
			return fmt.Errorf("error build request form for method %s, %w", method, errFormData)
		}
	}

	if fieldsCount > 0 {
		if errFormClose := form.Close(); errFormClose != nil {
			return fmt.Errorf("error form close for method %s, %w", method, errFormClose)
		}
	}

	path := b.botAPIPath(method)
	u := b.url + "/bot" + token + path
	var debugURL string
	if b.isDebug {
		debugURL = b.url + "/bot***" + path
	}

	if b.isDebug && strings.ToLower(method) != "getupdates" {
		requestDebugData, _ := json.Marshal(params)
		b.debugHandler("request url: %s, payload: %s", debugURL, requestDebugData)
	}

	req, errRequest := http.NewRequestWithContext(ctx, http.MethodPost, u, http.NoBody)
	if errRequest != nil {
		return fmt.Errorf("error create request for method %s, %w", method, redactRequestError(errRequest, token))
	}

	if fieldsCount > 0 {
		req.Header.Set("Content-Type", form.FormDataContentType())
		req.ContentLength = requestBody.size()
		req.GetBody = requestBody.replayReader
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	if fieldsCount > 0 {
		req.Body = requestBody.initialReader()
	}
	client := b.client
	if requestBody.file != nil {
		client = b.clientForSpooledRequest()
	}
	if beforeDispatch != nil {
		beforeDispatch()
	}
	resp, errDo := client.Do(req)
	if errDo != nil {
		if requestBody.file != nil && isRequestCancellation(ctx, errDo) {
			requestBody.discardUnread()
		}
		return fmt.Errorf("error do request for method %s, %w", method, redactRequestError(errDo, token))
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			b.errorsHandler(fmt.Errorf("failed to close response body: %w", err))
		}
	}()

	body, errReadBody := io.ReadAll(resp.Body)
	if errReadBody != nil {
		return fmt.Errorf("error read response body for method %s, %w", method, errReadBody)
	}

	r := apiResponse{}

	errDecode := json.Unmarshal(body, &r)
	if errDecode != nil {
		return fmt.Errorf("error decode response body for method %s, %s, %w", method, body, errDecode)
	}

	if !r.OK {
		switch r.ErrorCode {
		case http.StatusForbidden:
			return fmt.Errorf("%w, %s", ErrorForbidden, r.Description)
		case http.StatusBadRequest:
			if r.Parameters.MigrateToChatID != 0 {
				err := &MigrateError{
					Message:         fmt.Sprintf("%s: %s", ErrorBadRequest, r.Description),
					MigrateToChatID: r.Parameters.MigrateToChatID,
				}

				return err
			}
			return fmt.Errorf("%w, %s", ErrorBadRequest, r.Description)
		case http.StatusUnauthorized:
			err := fmt.Errorf("%w, %s", ErrorUnauthorized, r.Description)
			if method == "getUpdates" {
				return unauthorizedPollError{err}
			}
			return err
		case http.StatusNotFound:
			return fmt.Errorf("%w, %s", ErrorNotFound, r.Description)
		case http.StatusConflict:
			if method == "getUpdates" && r.Description == webhookActiveDescription {
				return errWebhookActive
			}
			return fmt.Errorf("%w, %s", ErrorConflict, r.Description)
		case http.StatusTooManyRequests:
			err := &TooManyRequestsError{
				Message:    fmt.Sprintf("%s, %s", ErrorTooManyRequests, r.Description),
				RetryAfter: r.Parameters.RetryAfter,
			}
			return err
		default:
			return fmt.Errorf("error response from telegram for method %s, %d %s", method, r.ErrorCode, r.Description)
		}
	}

	if !bytes.Equal(r.Result, []byte("[]")) {
		if b.isDebug {
			b.debugHandler("response from '%s' with payload '%s'", debugURL, body)
		}
	}

	if dest != nil {
		errDecodeDest := json.Unmarshal(r.Result, dest)
		if errDecodeDest != nil {
			return fmt.Errorf("error decode response result for method %s, %w", method, errDecodeDest)
		}
	}

	return nil
}

func isRequestCancellation(ctx context.Context, err error) bool {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// http.Client.Timeout on older Go versions exposes Timeout but no context sentinel.
	var timeoutErr net.Error
	return errors.As(err, &timeoutErr) && timeoutErr.Timeout()
}

func (b *Bot) clientForSpooledRequest() HttpClient {
	client, ok := b.client.(*http.Client)
	if !ok || client == nil {
		return b.client
	}
	redirectClient := *client
	checkRedirect := redirectClient.CheckRedirect
	if checkRedirect == nil {
		checkRedirect = defaultCheckRedirect
	}
	redirectClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		body, isSpooled := req.Body.(*multipartFileReader)
		err := checkRedirect(req, via)
		// net/http does not close the proposed request body when the callback rejects it.
		if err != nil && isSpooled {
			if closeErr := body.Close(); closeErr != nil {
				b.error("close rejected upload replay: %w", closeErr)
			}
		}
		return err
	}
	return &redirectClient
}

// defaultCheckRedirect mirrors the unexported defaultCheckRedirect in
// net/http/client.go, which http.Client uses when CheckRedirect is nil.
func defaultCheckRedirect(_ *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	return nil
}

func redactRequestError(err error, token string) error {
	if token == "" {
		return err
	}
	replacements := []string{token, "***"}
	if id, secret, ok := strings.Cut(token, ":"); ok {
		replacements = append(replacements, id+"%3A"+secret, "***", id+"%3a"+secret, "***")
	}
	redactor := requestErrorRedactor{replace: strings.NewReplacer(replacements...), keepURLProjections: true}
	clean, _ := redactor.redact(err)
	return clean
}

type requestErrorRedactor struct {
	replace            *strings.Replacer
	keepURLProjections bool
}

func (r requestErrorRedactor) redact(err error) (error, bool) {
	if err == nil {
		return nil, false
	}
	if urlErr, ok := err.(*url.Error); ok {
		clean, fieldsChanged := r.redactURLFields(urlErr)
		cause, causeChanged := r.redact(clean.Err)
		if fieldsChanged || causeChanged {
			clean.Err = cause
			return &clean, true
		}
		return err, false
	}

	original := err.Error()
	message := original
	clean := &redactedRequestError{original: err}
	var causeChanged bool
	joined, isJoined := err.(interface{ Unwrap() []error })
	if isJoined {
		causes := joined.Unwrap()
		clean.causes = make([]error, len(causes))
		for i, cause := range causes {
			var changed bool
			clean.causes[i], changed = r.redactNested(cause, &message)
			causeChanged = causeChanged || changed
		}
	} else if cause := errors.Unwrap(err); cause != nil {
		var safeCause error
		safeCause, causeChanged = r.redactNested(cause, &message)
		clean.causes = []error{safeCause}
	}
	var projectionChanged bool
	clean.projectedURL, projectionChanged = r.redactURLProjection(err, &message)
	message = r.replace.Replace(message)
	if message == original && !causeChanged && !projectionChanged {
		return err, false
	}
	clean.message = message
	if network, ok := err.(net.Error); ok {
		classified := &redactedNetworkError{clean, network}
		if isJoined {
			return &redactedJoinedNetworkError{classified}, true
		}
		return classified, true
	}
	if isJoined {
		return &redactedJoinedError{clean}, true
	}
	return clean, true
}

func (r requestErrorRedactor) redactNested(cause error, message *string) (error, bool) {
	clean, changed := r.redact(cause)
	if changed {
		// Replace the formatted cause to cover URL characters escaped by Error.
		*message = strings.ReplaceAll(*message, cause.Error(), clean.Error())
	}
	return clean, changed
}

func (r requestErrorRedactor) redactURLFields(err *url.Error) (url.Error, bool) {
	clean := *err
	clean.Op = r.replace.Replace(clean.Op)
	clean.URL = r.replace.Replace(clean.URL)
	return clean, clean.Op != err.Op || clean.URL != err.URL
}

func (r requestErrorRedactor) redactURLProjection(err error, message *string) (*url.Error, bool) {
	adapter, ok := err.(interface{ As(any) bool })
	if !ok {
		return nil, false
	}
	var projected *url.Error
	if !adapter.As(&projected) || projected == nil {
		return nil, false
	}
	// A projection can wrap the adapter that created it. Follow only ordinary
	// unwrap causes inside the projection and hide nested As projections.
	if !r.keepURLProjections {
		return nil, true
	}
	r.keepURLProjections = false
	clean, changed := r.redactURLFields(projected)
	cause, causeChanged := r.redact(clean.Err)
	clean.Err = cause
	changed = changed || causeChanged
	if changed {
		*message = strings.ReplaceAll(*message, projected.Error(), clean.Error())
	}
	return &clean, changed
}

// A custom transport wrapper can store credentials in fields the SDK cannot redact.
// Preserve the standard error contracts without returning the original wrapper.
type redactedRequestError struct {
	message      string
	original     error
	causes       []error
	projectedURL *url.Error
}

func (e *redactedRequestError) Error() string { return e.message }

func (e *redactedRequestError) GoString() string { return e.message }

func (e *redactedRequestError) Is(target error) bool { return errors.Is(e.original, target) }

func (e *redactedRequestError) Unwrap() error {
	if len(e.causes) == 0 {
		return nil
	}
	return e.causes[0]
}

func (e *redactedRequestError) As(target any) bool {
	switch result := target.(type) {
	case **url.Error:
		if e.projectedURL != nil {
			*result = e.projectedURL
			return true
		}
		return false
	case *net.Error:
		for _, cause := range e.causes {
			if errors.As(cause, result) {
				return true
			}
		}
		var network net.Error
		if errors.As(e.original, &network) {
			*result = &redactedNetworkError{e, network}
			return true
		}
	}
	return false
}

type redactedJoinedError struct{ *redactedRequestError }

func (e *redactedJoinedError) Unwrap() []error { return e.causes }

// Keep legacy Temporary classification for callers using net.Error on retries.
type networkErrorStatus interface {
	Timeout() bool
	Temporary() bool
}

type redactedNetworkError struct {
	*redactedRequestError
	status networkErrorStatus
}

func (e *redactedNetworkError) Timeout() bool { return e.status.Timeout() }

func (e *redactedNetworkError) Temporary() bool { return e.status.Temporary() }

type redactedJoinedNetworkError struct{ *redactedNetworkError }

func (e *redactedJoinedNetworkError) Unwrap() []error { return e.causes }
