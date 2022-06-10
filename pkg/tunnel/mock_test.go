package tunnel

import (
	"io"
	"sync"
)

// mocktail:Backend

type readWriteCloseMock struct {
	closedMu sync.Mutex
	closed   bool
}

func (r *readWriteCloseMock) Read(_ []byte) (n int, err error) {
	r.closedMu.Lock()
	defer r.closedMu.Unlock()

	if r.closed {
		return 0, io.EOF
	}

	return 0, nil
}

func (r *readWriteCloseMock) Write(_ []byte) (n int, err error) {
	return
}

func (r *readWriteCloseMock) Close() error {
	r.closedMu.Lock()
	r.closed = true
	r.closedMu.Unlock()

	return nil
}
