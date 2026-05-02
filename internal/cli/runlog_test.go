package cli

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

func TestRunLogBufferKeepsLatestTail(t *testing.T) {
	var buffer runLogBuffer
	prefix := []byte("prefix")
	tail := bytes.Repeat([]byte("x"), maxRunLogBytes)
	input := append(prefix, tail...)
	if n, err := buffer.Write(input); err != nil || n != len(input) {
		t.Fatalf("Write returned %d, %v", n, err)
	}
	if got := buffer.String(); got != string(tail) {
		t.Fatalf("tail length=%d match=%v", len(got), got == string(tail))
	}
	if !buffer.Truncated() {
		t.Fatal("buffer should be truncated")
	}
}

func TestRunLogBufferDropsOverflow(t *testing.T) {
	var buffer runLogBuffer
	first := bytes.Repeat([]byte("a"), maxRunLogBytes-2)
	second := []byte("bcde")
	if _, err := buffer.Write(first); err != nil {
		t.Fatal(err)
	}
	if buffer.Truncated() {
		t.Fatal("buffer should not be truncated before overflow")
	}
	if _, err := buffer.Write(second); err != nil {
		t.Fatal(err)
	}
	want := string(append(first[2:], second...))
	if got := buffer.String(); got != want {
		t.Fatalf("tail length=%d want=%d", len(got), len(want))
	}
	if !buffer.Truncated() {
		t.Fatal("buffer should be truncated after overflow")
	}
}

func TestRunLogBufferConcurrentWrites(t *testing.T) {
	var buffer runLogBuffer
	var wg sync.WaitGroup
	for _, text := range []string{"stdout-line\n", "stderr-line\n"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				if _, err := buffer.Write([]byte(text)); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	log := buffer.String()
	if !strings.Contains(log, "stdout-line\n") || !strings.Contains(log, "stderr-line\n") {
		t.Fatalf("log missing expected output: %q", log)
	}
}
