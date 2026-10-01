//go:build windows

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/status"
)

type pipeRequestResult struct {
	body []byte
	err  error
}

func startListOptionalInstallsPipeRequest(t *testing.T, sr *serviceRunner, payload string) <-chan pipeRequestResult {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "catalog-pipe-*.json")
	if err != nil {
		t.Fatal(err)
	}
	req := serviceEnvelope[json.RawMessage]{
		Version:      pipeProtocolVersion,
		MessageType:  messageTypeRequest,
		Operation:    actionListOptionalInstalls,
		RequestID:    newRequestID(),
		OperationID:  "",
		TimestampUTC: nowRFC3339UTC(),
		Payload:      json.RawMessage(payload),
	}
	encoded, err := json.Marshal(req)
	if err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	if _, err := file.Write(encoded); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}

	result := make(chan pipeRequestResult, 1)
	go func() {
		defer file.Close()
		sr.handlePipeCommand(context.Background(), file)
		if _, err := file.Seek(int64(len(encoded)), io.SeekStart); err != nil {
			result <- pipeRequestResult{err: err}
			return
		}
		body, err := io.ReadAll(file)
		result <- pipeRequestResult{body: body, err: err}
	}()
	return result
}

func awaitPipeRequest(t *testing.T, result <-chan pipeRequestResult) []byte {
	t.Helper()
	select {
	case out := <-result:
		if out.err != nil {
			t.Fatal(out.err)
		}
		return out.body
	case <-time.After(750 * time.Millisecond):
		t.Fatal("pipe request did not complete promptly")
		return nil
	}
}

func decodeListOptionalInstallsResponse(t *testing.T, body []byte) listOptionalInstallsResponse {
	t.Helper()
	var envelope serviceEnvelope[json.RawMessage]
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode response envelope: %v; body=%q", err, body)
	}
	if envelope.MessageType != messageTypeResponse {
		t.Fatalf("messageType = %q, want Response; body=%q", envelope.MessageType, body)
	}
	var payload listOptionalInstallsResponse
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		t.Fatalf("decode response payload: %v; body=%q", err, body)
	}
	return payload
}

func decodePipeError(t *testing.T, body []byte) errorResponsePayload {
	t.Helper()
	var envelope serviceEnvelope[json.RawMessage]
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode error envelope: %v; body=%q", err, body)
	}
	if envelope.MessageType != messageTypeError {
		t.Fatalf("messageType = %q, want Error; body=%q", envelope.MessageType, body)
	}
	var payload errorResponsePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		t.Fatalf("decode error payload: %v; body=%q", err, body)
	}
	return payload
}

func TestSnapshotListReturnsWhileExecMutexIsHeld(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	sr := newServiceRunner(cfg, nil)
	sr.catalogSnapshot = testSnapshot(t, cfg, "Snapshot A")

	sr.execMutex.Lock()
	defer sr.execMutex.Unlock()

	body := awaitPipeRequest(t, startListOptionalInstallsPipeRequest(t, sr, `{"refresh":false}`))
	payload := decodeListOptionalInstallsResponse(t, body)
	if payload.SnapshotAvailable == nil || !*payload.SnapshotAvailable {
		t.Fatalf("snapshotAvailable = %v, want true", payload.SnapshotAvailable)
	}
	if len(payload.Items) != 1 || payload.Items[0].ItemName != "Snapshot A" {
		t.Fatalf("items = %+v, want Snapshot A", payload.Items)
	}
	if payload.SnapshotGeneratedAtUTC != sr.catalogSnapshot.GeneratedAtUTC.Format(time.RFC3339) {
		t.Fatalf("snapshotGeneratedAtUtc = %q, want stored generation time %q", payload.SnapshotGeneratedAtUTC, sr.catalogSnapshot.GeneratedAtUTC.Format(time.RFC3339))
	}
	if sr.execMutex.TryLock() {
		sr.execMutex.Unlock()
		t.Fatal("execMutex was not still held when snapshot response completed")
	}
}

func TestListPayloadIsDecodedBeforeLegacyFallback(t *testing.T) {
	sr := newServiceRunner(config.Configuration{AppDataPath: t.TempDir()}, nil)
	body := awaitPipeRequest(t, startListOptionalInstallsPipeRequest(t, sr, `[]`))
	payload := decodePipeError(t, body)
	if payload.ErrorCode != "invalid_request" {
		t.Fatalf("errorCode = %q, want invalid_request", payload.ErrorCode)
	}
}

func TestSnapshotListToleratesUnknownRequestProperties(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	sr := newServiceRunner(cfg, nil)
	sr.catalogSnapshot = testSnapshot(t, cfg, "Snapshot A")
	body := awaitPipeRequest(t, startListOptionalInstallsPipeRequest(t, sr, `{"refresh":false,"futureProperty":"ignored"}`))
	payload := decodeListOptionalInstallsResponse(t, body)
	if payload.SnapshotAvailable == nil || !*payload.SnapshotAvailable {
		t.Fatalf("snapshotAvailable = %v, want true", payload.SnapshotAvailable)
	}
}

func TestSnapshotListDistinguishesMissingAndAuthoritativeEmptySnapshot(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}

	t.Run("missing", func(t *testing.T) {
		sr := newServiceRunner(cfg, nil)
		body := awaitPipeRequest(t, startListOptionalInstallsPipeRequest(t, sr, `{"refresh":false}`))
		payload := decodeListOptionalInstallsResponse(t, body)
		if payload.SnapshotAvailable == nil || *payload.SnapshotAvailable {
			t.Fatalf("snapshotAvailable = %v, want false", payload.SnapshotAvailable)
		}
		if payload.Items == nil || len(payload.Items) != 0 {
			t.Fatalf("items = %#v, want non-nil empty array", payload.Items)
		}
		if payload.SnapshotGeneratedAtUTC != "" {
			t.Fatalf("snapshotGeneratedAtUtc = %q, want omitted", payload.SnapshotGeneratedAtUTC)
		}
	})

	t.Run("authoritative empty", func(t *testing.T) {
		sr := newServiceRunner(cfg, nil)
		snapshot := testSnapshot(t, cfg, "unused")
		snapshot.Items = []optionalInstallResponseItem{}
		sr.catalogSnapshot = snapshot
		body := awaitPipeRequest(t, startListOptionalInstallsPipeRequest(t, sr, `{"refresh":false}`))
		payload := decodeListOptionalInstallsResponse(t, body)
		if payload.SnapshotAvailable == nil || !*payload.SnapshotAvailable {
			t.Fatalf("snapshotAvailable = %v, want true", payload.SnapshotAvailable)
		}
		if payload.Items == nil || len(payload.Items) != 0 {
			t.Fatalf("items = %#v, want non-nil empty array", payload.Items)
		}
		if payload.SnapshotGeneratedAtUTC != snapshot.GeneratedAtUTC.Format(time.RFC3339) {
			t.Fatalf("snapshotGeneratedAtUtc = %q, want %q", payload.SnapshotGeneratedAtUTC, snapshot.GeneratedAtUTC.Format(time.RFC3339))
		}
	})
}

func TestSnapshotListSerializesSafeRefreshStates(t *testing.T) {
	requested := time.Date(2026, 9, 30, 18, 13, 10, 0, time.UTC)
	completed := requested.Add(time.Minute)
	states := []catalogRefreshStatus{catalogRefreshIdle, catalogRefreshQueued, catalogRefreshRunning, catalogRefreshFailed}

	for _, state := range states {
		t.Run(string(state), func(t *testing.T) {
			cfg := config.Configuration{AppDataPath: t.TempDir()}
			sr := newServiceRunner(cfg, nil)
			sr.catalogSnapshot = testSnapshot(t, cfg, "Snapshot A")
			sr.catalogRefresh = catalogRefreshState{
				Status:         state,
				RequestedAtUTC: requested,
				CompletedAtUTC: completed,
				LastError:      "https://user:credential@example.test/private?token=super-secret",
			}

			body := awaitPipeRequest(t, startListOptionalInstallsPipeRequest(t, sr, `{"refresh":false}`))
			if bytes.Contains(body, []byte("credential")) || bytes.Contains(body, []byte("super-secret")) {
				t.Fatalf("raw refresh error leaked onto wire: %s", body)
			}
			payload := decodeListOptionalInstallsResponse(t, body)
			if payload.RefreshState != string(state) {
				t.Fatalf("refreshState = %q, want %q", payload.RefreshState, state)
			}
			if payload.RefreshRequestedAtUTC != requested.Format(time.RFC3339) || payload.RefreshCompletedAtUTC != completed.Format(time.RFC3339) {
				t.Fatalf("refresh timestamps drifted: %+v", payload)
			}
			if state == catalogRefreshFailed {
				if payload.RefreshErrorCode != "refresh_failed" {
					t.Fatalf("refreshErrorCode = %q, want refresh_failed", payload.RefreshErrorCode)
				}
			} else if payload.RefreshErrorCode != "" {
				t.Fatalf("refreshErrorCode = %q for state %s", payload.RefreshErrorCode, state)
			}
		})
	}
}

func TestRefreshTrueReturnsCurrentSnapshotBeforeBlockedGeneration(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir(), Catalogs: []string{"primary"}}
	stubOptionalCatalog(t,
		[]manifest.Item{{OptionalInstalls: []string{"Snapshot B"}}},
		map[int]map[string]catalog.Item{1: {"Snapshot B": {DisplayName: "Snapshot B", Installer: catalog.InstallerItem{Type: "msi", Location: "b.msi"}}}},
		map[string]status.Observation{"Snapshot B": {State: status.Absent, CheckedAtUTC: time.Now().UTC()}},
	)
	sr := newServiceRunner(cfg, nil)
	sr.catalogSnapshot = testSnapshot(t, cfg, "Snapshot A")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sr.wg.Add(1)
	go func() { defer sr.wg.Done(); sr.catalogRefreshWorker(ctx) }()

	sr.execMutex.Lock()
	locked := true
	defer func() {
		if locked {
			sr.execMutex.Unlock()
		}
		cancel()
		sr.wg.Wait()
	}()

	body := awaitPipeRequest(t, startListOptionalInstallsPipeRequest(t, sr, `{"refresh":true}`))
	payload := decodeListOptionalInstallsResponse(t, body)
	if len(payload.Items) != 1 || payload.Items[0].ItemName != "Snapshot A" {
		t.Fatalf("immediate response = %+v, want Snapshot A", payload.Items)
	}
	if payload.RefreshState != string(catalogRefreshQueued) {
		t.Fatalf("refreshState = %q, want Queued while execMutex is held", payload.RefreshState)
	}

	sr.execMutex.Unlock()
	locked = false
	waitForRefreshStatus(t, sr, catalogRefreshIdle)
	fresh, ok := sr.currentCatalogSnapshot()
	if !ok || len(fresh.Items) != 1 || fresh.Items[0].ItemName != "Snapshot B" {
		t.Fatalf("background refresh did not publish Snapshot B: %#v", fresh)
	}
}

func TestRefreshTrueWithoutSnapshotReturnsImmediatelyDuringBlockedStartup(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir(), Catalogs: []string{"primary"}}
	stubOptionalCatalog(t, []manifest.Item{}, map[int]map[string]catalog.Item{}, map[string]status.Observation{})
	sr := newServiceRunner(cfg, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sr.wg.Add(1)
	go func() { defer sr.wg.Done(); sr.catalogRefreshWorker(ctx) }()

	sr.execMutex.Lock()
	locked := true
	defer func() {
		if locked {
			sr.execMutex.Unlock()
		}
		cancel()
		sr.wg.Wait()
	}()

	body := awaitPipeRequest(t, startListOptionalInstallsPipeRequest(t, sr, `{"refresh":true}`))
	payload := decodeListOptionalInstallsResponse(t, body)
	if payload.SnapshotAvailable == nil || *payload.SnapshotAvailable {
		t.Fatalf("snapshotAvailable = %v, want false", payload.SnapshotAvailable)
	}
	if payload.Items == nil || len(payload.Items) != 0 {
		t.Fatalf("items = %#v, want empty", payload.Items)
	}
	if payload.RefreshState != string(catalogRefreshQueued) {
		t.Fatalf("refreshState = %q, want Queued", payload.RefreshState)
	}

	sr.execMutex.Unlock()
	locked = false
	waitForRefreshStatus(t, sr, catalogRefreshIdle)
}

func TestPersistedSnapshotIsReadableWhileStartupExecutionIsBlocked(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	persisted := testSnapshot(t, cfg, "Persisted")
	if err := persistCatalogSnapshot(cfg, persisted); err != nil {
		t.Fatal(err)
	}

	sr := newServiceRunner(cfg, nil)
	sr.loadPersistedCatalogSnapshot()
	sr.execMutex.Lock()
	defer sr.execMutex.Unlock()

	body := awaitPipeRequest(t, startListOptionalInstallsPipeRequest(t, sr, `{"refresh":false}`))
	payload := decodeListOptionalInstallsResponse(t, body)
	if payload.SnapshotAvailable == nil || !*payload.SnapshotAvailable || len(payload.Items) != 1 || payload.Items[0].ItemName != "Persisted" {
		t.Fatalf("persisted snapshot was not served during blocked startup: %+v", payload)
	}
}

func TestLegacyListStillWaitsWhileSnapshotListReturns(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	sr := newServiceRunner(cfg, nil)
	sr.catalogSnapshot = testSnapshot(t, cfg, "Snapshot A")

	sr.execMutex.Lock()
	locked := true
	defer func() {
		if locked {
			sr.execMutex.Unlock()
		}
	}()

	workerDone := make(chan struct{})
	go func() {
		queued := <-sr.queue
		sr.execMutex.Lock()
		resp := CommandResponse{Status: "ok"}
		sr.execMutex.Unlock()
		queued.result <- queuedResult{resp: resp}
		close(workerDone)
	}()

	legacy := startListOptionalInstallsPipeRequest(t, sr, `{}`)
	select {
	case out := <-legacy:
		t.Fatalf("legacy request completed while execMutex was held: err=%v body=%s", out.err, out.body)
	case <-time.After(100 * time.Millisecond):
	}

	fastBody := awaitPipeRequest(t, startListOptionalInstallsPipeRequest(t, sr, `{"refresh":false}`))
	fast := decodeListOptionalInstallsResponse(t, fastBody)
	if fast.SnapshotAvailable == nil || !*fast.SnapshotAvailable || len(fast.Items) != 1 || fast.Items[0].ItemName != "Snapshot A" {
		t.Fatalf("snapshot request did not return current snapshot: %+v", fast)
	}
	select {
	case out := <-legacy:
		t.Fatalf("legacy request completed before execMutex release: err=%v body=%s", out.err, out.body)
	default:
	}

	sr.execMutex.Unlock()
	locked = false
	legacyBody := awaitPipeRequest(t, legacy)
	legacyPayload := decodeListOptionalInstallsResponse(t, legacyBody)
	if legacyPayload.SnapshotAvailable != nil {
		t.Fatalf("legacy response unexpectedly included snapshot metadata: %+v", legacyPayload)
	}
	select {
	case <-workerDone:
	case <-time.After(time.Second):
		t.Fatal("legacy queue worker did not finish")
	}
}

func TestConcurrentRefreshRequestsCoalesceThroughPipe(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir(), Catalogs: []string{"primary"}}
	stubOptionalCatalog(t,
		[]manifest.Item{{OptionalInstalls: []string{"Snapshot B"}}},
		map[int]map[string]catalog.Item{1: {"Snapshot B": {DisplayName: "Snapshot B", Installer: catalog.InstallerItem{Type: "msi", Location: "b.msi"}}}},
		map[string]status.Observation{"Snapshot B": {State: status.Absent, CheckedAtUTC: time.Now().UTC()}},
	)
	stubManifestGet := manifestGet
	var projections int32
	manifestGet = func(cfg config.Configuration) ([]manifest.Item, []string, error) {
		atomic.AddInt32(&projections, 1)
		return stubManifestGet(cfg)
	}

	sr := newServiceRunner(cfg, nil)
	sr.catalogSnapshot = testSnapshot(t, cfg, "Snapshot A")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sr.wg.Add(1)
	go func() { defer sr.wg.Done(); sr.catalogRefreshWorker(ctx) }()

	sr.execMutex.Lock()
	locked := true
	defer func() {
		if locked {
			sr.execMutex.Unlock()
		}
		cancel()
		sr.wg.Wait()
	}()

	const requestCount = 8
	results := make([]<-chan pipeRequestResult, requestCount)
	for i := range results {
		results[i] = startListOptionalInstallsPipeRequest(t, sr, `{"refresh":true}`)
	}
	for i, result := range results {
		payload := decodeListOptionalInstallsResponse(t, awaitPipeRequest(t, result))
		if len(payload.Items) != 1 || payload.Items[0].ItemName != "Snapshot A" {
			t.Fatalf("request %d returned %+v, want Snapshot A", i, payload.Items)
		}
		if payload.RefreshState != string(catalogRefreshQueued) {
			t.Fatalf("request %d refreshState = %q, want Queued", i, payload.RefreshState)
		}
	}

	sr.execMutex.Unlock()
	locked = false
	waitForRefreshStatus(t, sr, catalogRefreshIdle)
	if got := atomic.LoadInt32(&projections); got != 1 {
		t.Fatalf("background projection count = %d, want 1", got)
	}
	fresh, ok := sr.currentCatalogSnapshot()
	if !ok || len(fresh.Items) != 1 || fresh.Items[0].ItemName != "Snapshot B" {
		t.Fatalf("coalesced refresh did not publish Snapshot B: %#v", fresh)
	}
}

func TestSnapshotListRefreshTimestampsOmitZeroValues(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	sr := newServiceRunner(cfg, nil)
	sr.catalogRefresh = catalogRefreshState{Status: catalogRefreshIdle}
	body := awaitPipeRequest(t, startListOptionalInstallsPipeRequest(t, sr, `{"refresh":false}`))
	if bytes.Contains(body, []byte("refreshRequestedAtUtc")) || bytes.Contains(body, []byte("refreshCompletedAtUtc")) {
		t.Fatalf("zero refresh timestamps should be omitted: %s", body)
	}
	payload := decodeListOptionalInstallsResponse(t, body)
	if payload.RefreshState != string(catalogRefreshIdle) {
		t.Fatalf("refreshState = %q, want Idle", payload.RefreshState)
	}
}

func TestSnapshotListResponseUsesProtocolV1(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	sr := newServiceRunner(cfg, nil)
	body := awaitPipeRequest(t, startListOptionalInstallsPipeRequest(t, sr, `{"refresh":false}`))
	var envelope serviceEnvelope[json.RawMessage]
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Version != "v1" {
		t.Fatalf("version = %q, want v1", envelope.Version)
	}
	if envelope.Operation != actionListOptionalInstalls {
		t.Fatalf("operation = %q, want %q", envelope.Operation, actionListOptionalInstalls)
	}
}

func TestSnapshotListDoesNotUseResponseTimeAsGenerationTime(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	sr := newServiceRunner(cfg, nil)
	snapshot := testSnapshot(t, cfg, "Snapshot A")
	snapshot.GeneratedAtUTC = time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	sr.catalogSnapshot = snapshot
	body := awaitPipeRequest(t, startListOptionalInstallsPipeRequest(t, sr, `{"refresh":false}`))
	payload := decodeListOptionalInstallsResponse(t, body)
	if payload.SnapshotGeneratedAtUTC != "2025-01-02T03:04:05Z" {
		t.Fatalf("snapshotGeneratedAtUtc = %q, want stored snapshot timestamp", payload.SnapshotGeneratedAtUTC)
	}
}
