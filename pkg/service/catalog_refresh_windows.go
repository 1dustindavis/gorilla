//go:build windows

package service

import (
	"context"
	"time"

	"github.com/1dustindavis/gorilla/pkg/gorillalog"
)

type catalogRefreshStatus string

const (
	catalogRefreshIdle    catalogRefreshStatus = "Idle"
	catalogRefreshQueued  catalogRefreshStatus = "Queued"
	catalogRefreshRunning catalogRefreshStatus = "Running"
	catalogRefreshFailed  catalogRefreshStatus = "Failed"
)

type catalogRefreshState struct {
	Status         catalogRefreshStatus
	RequestedAtUTC time.Time
	CompletedAtUTC time.Time
	LastError      string
}

func (sr *serviceRunner) requestCatalogRefresh() {
	now := time.Now().UTC()
	sr.catalogRefreshMu.Lock()
	if sr.catalogRefresh.Status == catalogRefreshQueued || sr.catalogRefresh.Status == catalogRefreshRunning {
		sr.catalogRefreshMu.Unlock()
		gorillalog.Debug("catalog refresh coalesced")
		return
	}
	sr.catalogRefresh.Status = catalogRefreshQueued
	sr.catalogRefresh.RequestedAtUTC = now
	sr.catalogRefresh.CompletedAtUTC = time.Time{}
	sr.catalogRefresh.LastError = ""
	sr.catalogRefreshMu.Unlock()

	select {
	case sr.catalogSignal <- struct{}{}:
		gorillalog.Debug("catalog refresh requested")
	default:
		gorillalog.Debug("catalog refresh coalesced")
	}
}

func (sr *serviceRunner) catalogRefreshWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-sr.catalogSignal:
		}

		sr.catalogRefreshMu.Lock()
		requestedAt := sr.catalogRefresh.RequestedAtUTC
		if sr.catalogRefresh.Status != catalogRefreshQueued {
			sr.catalogRefreshMu.Unlock()
			continue
		}
		sr.catalogRefreshMu.Unlock()

		sr.execMutex.Lock()
		if snapshot, ok := sr.currentCatalogSnapshot(); ok && !snapshot.GeneratedAtUTC.Before(requestedAt) {
			sr.catalogRefreshMu.Lock()
			sr.catalogRefresh.Status = catalogRefreshIdle
			sr.catalogRefresh.CompletedAtUTC = time.Now().UTC()
			sr.catalogRefresh.LastError = ""
			sr.catalogRefreshMu.Unlock()
			sr.execMutex.Unlock()
			gorillalog.Debug("catalog refresh satisfied by managed-run publication")
			continue
		}

		sr.catalogRefreshMu.Lock()
		sr.catalogRefresh.Status = catalogRefreshRunning
		sr.catalogRefreshMu.Unlock()

		details, err := getOptionalItemDetails(sr.cfg)
		if err == nil {
			err = sr.publishCatalogSnapshot(details, sr.cfg, catalogSnapshotBackgroundRefresh)
			// Persistence failure does not make the refresh projection invalid. The
			// in-memory snapshot was already published and remains useful.
			if err != nil {
				gorillalog.Warn("catalog snapshot persistence failed after background refresh:", err)
				err = nil
			}
		}
		sr.execMutex.Unlock()

		sr.catalogRefreshMu.Lock()
		sr.catalogRefresh.CompletedAtUTC = time.Now().UTC()
		if err != nil {
			sr.catalogRefresh.Status = catalogRefreshFailed
			sr.catalogRefresh.LastError = err.Error()
			gorillalog.Warn("catalog projection failed:", "trigger=", catalogSnapshotBackgroundRefresh, "error=", err)
		} else {
			sr.catalogRefresh.Status = catalogRefreshIdle
			sr.catalogRefresh.LastError = ""
		}
		sr.catalogRefreshMu.Unlock()
	}
}

func (sr *serviceRunner) noteCatalogPublication(generatedAt time.Time) {
	sr.catalogRefreshMu.Lock()
	defer sr.catalogRefreshMu.Unlock()
	if sr.catalogRefresh.Status == catalogRefreshQueued && !generatedAt.Before(sr.catalogRefresh.RequestedAtUTC) {
		sr.catalogRefresh.Status = catalogRefreshIdle
		sr.catalogRefresh.CompletedAtUTC = generatedAt
		sr.catalogRefresh.LastError = ""
		gorillalog.Debug("catalog refresh satisfied by managed-run publication")
	}
}
