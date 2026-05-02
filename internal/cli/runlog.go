package cli

import "sync"

const maxRunLogBytes = 64 * 1024

type runLogBuffer struct {
	mu        sync.Mutex
	data      []byte
	truncated bool
}

func (b *runLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(p) >= maxRunLogBytes {
		b.data = append(b.data[:0], p[len(p)-maxRunLogBytes:]...)
		b.truncated = true
		return len(p), nil
	}
	overflow := len(b.data) + len(p) - maxRunLogBytes
	if overflow > 0 {
		copy(b.data, b.data[overflow:])
		b.data = b.data[:len(b.data)-overflow]
		b.truncated = true
	}
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *runLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}

func (b *runLogBuffer) Truncated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.truncated
}
