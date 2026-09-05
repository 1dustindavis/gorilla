//go:build windows

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/1dustindavis/gorilla/pkg/config"
	"golang.org/x/sys/windows"
)

func TestServiceRunnerStopUnblocksIdleNamedPipeListener(t *testing.T) {
	cfg := config.Configuration{
		AppDataPath:     t.TempDir(),
		ServicePipeName: fmt.Sprintf("gorilla-stop-test-%d", time.Now().UnixNano()),
		ServiceInterval: "1h",
		ServiceMode:     true,
		ServiceName:     "gorilla-stop-test",
	}

	sr := newServiceRunner(cfg, func(config.Configuration) error { return nil })
	ctx, cancel := context.WithCancel(context.Background())
	if err := sr.start(ctx); err != nil {
		t.Fatalf("service start failed: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		sr.pipeListenerMu.Lock()
		handle := sr.pipeListenerHandle
		sr.pipeListenerMu.Unlock()
		if handle != 0 && handle != windows.InvalidHandle {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("service did not enter idle named-pipe accept state")
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer stopCancel()

	started := time.Now()
	sr.stop(stopCtx)
	elapsed := time.Since(started)

	if stopCtx.Err() != nil {
		t.Fatalf("service shutdown hit its context deadline instead of unblocking the idle pipe listener: %v", stopCtx.Err())
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("service shutdown took %v; expected idle pipe listener to unblock promptly", elapsed)
	}
}
