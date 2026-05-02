package cli

import (
	"strings"
	"sync"
	"testing"
)

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
