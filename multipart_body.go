package bot

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
)

const multipartMemoryLimitBytes = 1 << 20

// multipartBody buffers small forms and spools larger forms to a temporary file.
// Once writing has finished, each reader starts at the beginning of the form.
type multipartBody struct {
	buffer   bytes.Buffer
	file     *os.File
	fileSize int64

	mu                sync.Mutex
	unreadReaders     int
	startedReaders    int
	isOwnerReleased   bool
	isUnreadDiscarded bool
}

func (b *multipartBody) Write(p []byte) (int, error) {
	if b.file == nil {
		// A browser's Go runtime has no filesystem for spooling.
		if runtime.GOOS == "js" || len(p) <= multipartMemoryLimitBytes-b.buffer.Len() {
			return b.buffer.Write(p)
		}
		file, err := createMultipartFile()
		if err != nil {
			return 0, fmt.Errorf("create temporary upload file: %w", err)
		}
		b.file = file
		b.fileSize, err = b.buffer.WriteTo(file)
		if err != nil {
			return 0, err
		}
		b.buffer = bytes.Buffer{}
	}
	n, err := b.file.Write(p)
	b.fileSize += int64(n)
	return n, err
}

func (b *multipartBody) initialReader() io.ReadCloser {
	if b.file == nil {
		return io.NopCloser(bytes.NewReader(b.buffer.Bytes()))
	}
	// The first body is handed to Do, which may return before beginning its read.
	b.unreadReaders++
	return newMultipartFileReader(b)
}

func (b *multipartBody) replayReader() (io.ReadCloser, error) {
	if b.file == nil {
		return io.NopCloser(bytes.NewReader(b.buffer.Bytes())), nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.isOwnerReleased || b.isUnreadDiscarded {
		return nil, os.ErrClosed
	}
	b.unreadReaders++
	return newMultipartFileReader(b), nil
}

func (b *multipartBody) size() int64 {
	if b.file == nil {
		return int64(b.buffer.Len())
	}
	return b.fileSize
}

func (b *multipartBody) Close() error {
	if b.file == nil {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.isOwnerReleased = true
	return b.closeIfUnused()
}

func (b *multipartBody) startReader() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.isUnreadDiscarded {
		return os.ErrClosed
	}
	b.unreadReaders--
	b.startedReaders++
	return nil
}

func (b *multipartBody) discardUnread() {
	b.mu.Lock()
	defer b.mu.Unlock()
	// HTTP/2 can reserve a replay before its retry delay and abandon it on cancellation.
	b.isUnreadDiscarded = true
	b.unreadReaders = 0
}

func (b *multipartBody) releaseReader(hasStarted bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if hasStarted {
		b.startedReaders--
	} else if b.isUnreadDiscarded {
		return nil
	} else {
		b.unreadReaders--
	}
	return b.closeIfUnused()
}

// closeIfUnused requires mu. Only the final lease or owner release closes the file.
func (b *multipartBody) closeIfUnused() error {
	if b.isOwnerReleased && b.unreadReaders == 0 && b.startedReaders == 0 {
		return b.file.Close()
	}
	return nil
}

// Each attempt reads its own section of the shared file until its body closes.
type multipartFileReader struct {
	body *multipartBody

	mu         sync.Mutex
	section    *io.SectionReader
	isClosed   bool
	hasStarted bool
}

func newMultipartFileReader(body *multipartBody) *multipartFileReader {
	r := &multipartFileReader{body: body, section: io.NewSectionReader(body.file, 0, body.fileSize)}
	// Fallback for replay bodies a transport abandons without closing.
	runtime.SetFinalizer(r, (*multipartFileReader).finalize)
	return r
}

func (r *multipartFileReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.isClosed {
		return 0, os.ErrClosed
	}
	if !r.hasStarted && len(p) != 0 {
		if err := r.body.startReader(); err != nil {
			return 0, err
		}
		r.hasStarted = true
	}
	return r.section.Read(p)
}

func (r *multipartFileReader) Close() error {
	runtime.SetFinalizer(r, nil)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.isClosed {
		return nil
	}
	r.isClosed = true
	return r.body.releaseReader(r.hasStarted)
}

func (r *multipartFileReader) finalize() {
	if err := r.Close(); err != nil {
		defaultErrorsHandler(fmt.Errorf("clean up abandoned request body: %w", err))
	}
}
