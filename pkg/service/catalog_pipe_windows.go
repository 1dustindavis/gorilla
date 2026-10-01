//go:build windows

package service

import (
	"encoding/json"
	"os"
	"time"

	"github.com/1dustindavis/gorilla/pkg/gorillalog"
)

type catalogReadView struct {
	Snapshot *optionalCatalogSnapshot
	Refresh  catalogRefreshState
}

// catalogReadView takes catalogRefreshMu before catalogMu so snapshot and
// refresh state form a coherent/conservative view. Publication replaces the
// snapshot under catalogMu, releases it, then calls noteCatalogPublication,
// which acquires catalogRefreshMu. Therefore a reader may observe a newly
// published snapshot while refresh still appears in progress, but it cannot
// observe refresh completion paired with the older snapshot.
func (sr *serviceRunner) catalogReadView() catalogReadView {
	sr.catalogRefreshMu.Lock()
	defer sr.catalogRefreshMu.Unlock()

	sr.catalogMu.RLock()
	defer sr.catalogMu.RUnlock()

	return catalogReadView{
		Snapshot: cloneCatalogSnapshot(sr.catalogSnapshot),
		Refresh:  sr.catalogRefresh,
	}
}

func boolPointer(value bool) *bool {
	return &value
}

func catalogRefreshTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func (sr *serviceRunner) writeCatalogSnapshotResponse(
	file *os.File,
	req serviceEnvelope[json.RawMessage],
	request listOptionalInstallsRequest,
) error {
	if request.Refresh != nil && *request.Refresh {
		sr.requestCatalogRefresh()
	}

	view := sr.catalogReadView()
	payload := listOptionalInstallsResponse{
		Items:                 []optionalInstallResponseItem{},
		SnapshotAvailable:     boolPointer(view.Snapshot != nil),
		RefreshState:          string(view.Refresh.Status),
		RefreshRequestedAtUTC: catalogRefreshTimestamp(view.Refresh.RequestedAtUTC),
		RefreshCompletedAtUTC: catalogRefreshTimestamp(view.Refresh.CompletedAtUTC),
	}
	if view.Snapshot != nil {
		payload.Items = view.Snapshot.Items
		payload.SnapshotGeneratedAtUTC = view.Snapshot.GeneratedAtUTC.UTC().Format(time.RFC3339)
	}
	if view.Refresh.Status == catalogRefreshFailed {
		payload.RefreshErrorCode = "refresh_failed"
	}

	if err := json.NewEncoder(file).Encode(serviceEnvelope[listOptionalInstallsResponse]{
		Version:      pipeProtocolVersion,
		MessageType:  messageTypeResponse,
		Operation:    actionListOptionalInstalls,
		RequestID:    req.RequestID,
		OperationID:  "",
		TimestampUTC: nowRFC3339UTC(),
		Payload:      payload,
	}); err != nil {
		return err
	}

	gorillalog.Debug(
		"ListOptionalInstalls snapshot response:",
		"refreshRequested=", request.Refresh != nil && *request.Refresh,
		"snapshotAvailable=", view.Snapshot != nil,
		"refreshState=", view.Refresh.Status,
	)
	return nil
}
