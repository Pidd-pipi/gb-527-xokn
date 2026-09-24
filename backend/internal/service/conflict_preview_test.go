package service

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"satellite-contact-window-deconfliction/backend/internal/config"
	"satellite-contact-window-deconfliction/backend/internal/constants"
	"satellite-contact-window-deconfliction/backend/internal/dto"
	"satellite-contact-window-deconfliction/backend/internal/model"
	"satellite-contact-window-deconfliction/backend/internal/repository"
)

type previewFixture struct {
	service  *ConflictResolutionService
	db       *gorm.DB
	windows  *repository.ContactWindowRepository
	stations *repository.GroundStationRepository
	actor    dto.Actor
	reviewer dto.Actor
	stationA model.GroundStation
	stationB model.GroundStation
	members  []model.ContactWindow
	pending  dto.ConflictResolutionResponse
}

func newPreviewFixture(t *testing.T, dsn string) previewFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.GroundStation{}, &model.SatelliteAsset{}, &model.ContactWindow{}, &model.ConflictResolution{}, &model.AuditEvent{}); err != nil {
		t.Fatal(err)
	}
	stations := []model.GroundStation{
		{StationCode: "PV-GS-A", Name: "Preview A", AntennaCount: 1, SupportedBandsJSON: `["S"]`, StationStatus: "active", Version: 1},
		{StationCode: "PV-GS-B", Name: "Preview B", AntennaCount: 2, SupportedBandsJSON: `["S"]`, StationStatus: "active", Version: 1},
	}
	if err := db.Create(&stations).Error; err != nil {
		t.Fatal(err)
	}
	assets := []model.SatelliteAsset{
		{SatelliteCode: "PV-SAT-A", Name: "A", SupportedBandsJSON: `["S"]`, MinimumContactSec: 60, AssetStatus: "active", PriorityWeight: 2, Version: 1},
		{SatelliteCode: "PV-SAT-B", Name: "B", SupportedBandsJSON: `["S"]`, MinimumContactSec: 60, AssetStatus: "active", PriorityWeight: 1, Version: 1},
	}
	if err := db.Create(&assets).Error; err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Truncate(time.Second)
	windows := []model.ContactWindow{
		{StationID: stations[0].ID, SatelliteID: assets[0].ID, StartAt: base, EndAt: base.Add(10 * time.Minute), Band: "S", WindowStatus: constants.WindowStatusSubmitted, Priority: 8, SourceVersion: "preview-source", Version: 1},
		{StationID: stations[0].ID, SatelliteID: assets[1].ID, StartAt: base.Add(time.Minute), EndAt: base.Add(9 * time.Minute), Band: "S", WindowStatus: constants.WindowStatusSubmitted, Priority: 5, SourceVersion: "preview-source", Version: 1},
		{StationID: stations[0].ID, SatelliteID: assets[0].ID, StartAt: base.Add(30 * time.Minute), EndAt: base.Add(40 * time.Minute), Band: "S", WindowStatus: constants.WindowStatusSubmitted, Priority: 6, SourceVersion: "preview-source", Version: 1},
	}
	if err := db.Create(&windows).Error; err != nil {
		t.Fatal(err)
	}
	windowRepository := repository.NewContactWindowRepository(db)
	stationRepository := repository.NewGroundStationRepository(db)
	service := NewConflictResolutionService(
		repository.NewConflictResolutionRepository(db), windowRepository, stationRepository, repository.NewSatelliteAssetRepository(db),
		NewAuditService(repository.NewSystemRepository(db)), config.Weights{PriorityLoss: 4, MovementDistance: .02, ContactDuration: .003, ResourceMargin: 2},
	)
	actor := dto.Actor{ID: 1, Username: "scheduler", Role: constants.RoleScheduler}
	detected, err := service.Detect(dto.DetectConflictsRequest{From: base.Add(-time.Minute).Format(time.RFC3339), To: base.Add(time.Hour).Format(time.RFC3339)}, actor, "preview-detect")
	if err != nil {
		t.Fatal(err)
	}
	var target dto.ConflictResolutionResponse
	for _, resolution := range detected.Resolutions {
		if resolution.ConflictType == constants.ConflictTypeStationCapacity {
			target = resolution
			break
		}
	}
	if target.ID == 0 {
		t.Fatalf("expected a station capacity conflict, got %+v", detected.Resolutions)
	}
	pending, err := service.Submit(target.ID, dto.ConflictActionRequest{ExpectedVersion: target.Version}, actor, "preview-submit")
	if err != nil {
		t.Fatal(err)
	}
	return previewFixture{
		service: service, db: db, windows: windowRepository, stations: stationRepository,
		actor: actor, reviewer: dto.Actor{ID: 2, Username: "reviewer", Role: constants.RoleReviewer},
		stationA: stations[0], stationB: stations[1], members: windows, pending: pending,
	}
}

func (fixture previewFixture) suggestion(t *testing.T, actionType string) dto.ResolutionSuggestion {
	t.Helper()
	for _, suggestion := range fixture.pending.Suggestions {
		if suggestion.ActionType == actionType {
			return suggestion
		}
	}
	t.Fatalf("expected a %s suggestion in %+v", actionType, fixture.pending.Suggestions)
	return dto.ResolutionSuggestion{}
}

func (fixture previewFixture) auditCount(t *testing.T) int64 {
	t.Helper()
	var count int64
	if err := fixture.db.Model(&model.AuditEvent{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func dispositionsByWindow(preview dto.ConflictPreviewResponse) map[uint]dto.PreviewDisposition {
	result := map[uint]dto.PreviewDisposition{}
	for _, item := range preview.Dispositions {
		result[item.WindowID] = item
	}
	return result
}

func TestPreviewSimulatesSelectedPlanWithoutSideEffects(t *testing.T) {
	fixture := newPreviewFixture(t, "file:conflict-preview-simulate?mode=memory&cache=shared")
	first, second := fixture.members[0], fixture.members[1]
	alternate := fixture.members[2]

	reassign := fixture.suggestion(t, "assign_compatible_station")
	preview, err := fixture.service.Preview(fixture.pending.ID, dto.ConflictPreviewRequest{ExpectedVersion: fixture.pending.Version, ActionKey: reassign.ActionKey})
	if err != nil {
		t.Fatal(err)
	}
	if preview.ActionKey != reassign.ActionKey || preview.ResolutionID != fixture.pending.ID {
		t.Fatalf("unexpected preview identity %+v", preview)
	}
	dispositions := dispositionsByWindow(preview)
	if got := dispositions[first.ID].Disposition; got != constants.PreviewDispositionReassigned {
		t.Fatalf("expected moved window %d to be reassigned, got %s", first.ID, got)
	}
	if target := dispositions[first.ID].TargetStationID; target == nil || *target != fixture.stationB.ID {
		t.Fatalf("expected reassignment target station %d, got %+v", fixture.stationB.ID, target)
	}
	if got := dispositions[second.ID].Disposition; got != constants.PreviewDispositionKept {
		t.Fatalf("expected untouched window %d to be kept, got %s", second.ID, got)
	}
	if preview.RemainingConflicts != 0 || len(preview.RemainingReasons) != 0 {
		t.Fatalf("expected no remaining conflicts after reassignment, got %+v", preview.RemainingReasons)
	}

	keep := fixture.suggestion(t, "keep_high_priority")
	preview, err = fixture.service.Preview(fixture.pending.ID, dto.ConflictPreviewRequest{ExpectedVersion: fixture.pending.Version, ActionKey: keep.ActionKey})
	if err != nil {
		t.Fatal(err)
	}
	dispositions = dispositionsByWindow(preview)
	if got := dispositions[first.ID].Disposition; got != constants.PreviewDispositionKept {
		t.Fatalf("expected top priority window %d to be kept, got %s", first.ID, got)
	}
	if got := dispositions[second.ID].Disposition; got != constants.PreviewDispositionManual {
		t.Fatalf("expected targetless moved window %d to need manual handling, got %s", second.ID, got)
	}
	if preview.RemainingConflicts != 1 || preview.RemainingReasons[0].ConflictType != constants.ConflictTypeStationCapacity {
		t.Fatalf("expected the original capacity conflict to remain, got %+v", preview.RemainingReasons)
	}

	alternateAction := fixture.suggestion(t, "use_alternate_window")
	preview, err = fixture.service.Preview(fixture.pending.ID, dto.ConflictPreviewRequest{ExpectedVersion: fixture.pending.Version, ActionKey: alternateAction.ActionKey})
	if err != nil {
		t.Fatal(err)
	}
	dispositions = dispositionsByWindow(preview)
	if got := dispositions[first.ID].Disposition; got != constants.PreviewDispositionUseAlternate {
		t.Fatalf("expected window %d to use the alternate window, got %s", first.ID, got)
	}
	if replacement := dispositions[first.ID].AlternateWindowID; replacement == nil || *replacement != alternate.ID {
		t.Fatalf("expected alternate window %d, got %+v", alternate.ID, replacement)
	}
	if preview.RemainingConflicts != 0 {
		t.Fatalf("expected alternate window to clear the conflict, got %+v", preview.RemainingReasons)
	}

	auditsBefore := fixture.auditCount(t)
	if _, err := fixture.service.Preview(fixture.pending.ID, dto.ConflictPreviewRequest{ExpectedVersion: fixture.pending.Version, ActionKey: reassign.ActionKey}); err != nil {
		t.Fatal(err)
	}
	if got := fixture.auditCount(t); got != auditsBefore {
		t.Fatalf("preview must not write audit events, count changed from %d to %d", auditsBefore, got)
	}
	resolution, err := fixture.service.Get(fixture.pending.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resolution.ResolutionStatus != constants.ResolutionStatusPendingReview || resolution.Version != fixture.pending.Version {
		t.Fatalf("preview must not change review state, got status %s version %d", resolution.ResolutionStatus, resolution.Version)
	}
	for _, window := range fixture.members {
		current, err := fixture.windows.Get(window.ID)
		if err != nil {
			t.Fatal(err)
		}
		if current.Version != window.Version {
			t.Fatalf("preview must not change window %d, version moved from %d to %d", window.ID, window.Version, current.Version)
		}
	}
}

func TestPreviewReportsBlockers(t *testing.T) {
	t.Run("window version changed", func(t *testing.T) {
		fixture := newPreviewFixture(t, "file:conflict-preview-window-changed?mode=memory&cache=shared")
		if _, err := fixture.windows.Update(fixture.members[0].ID, fixture.members[0].Version, map[string]any{"priority": 9}); err != nil {
			t.Fatal(err)
		}
		reassign := fixture.suggestion(t, "assign_compatible_station")
		_, err := fixture.service.Preview(fixture.pending.ID, dto.ConflictPreviewRequest{ExpectedVersion: fixture.pending.Version, ActionKey: reassign.ActionKey})
		blockers := assertPreviewBlocked(t, err, constants.PreviewBlockerWindow, "changed")
		if blockers[0].ID != fixture.members[0].ID {
			t.Fatalf("expected blocker for window %d, got %+v", fixture.members[0].ID, blockers[0])
		}
	})
	t.Run("target station inactive", func(t *testing.T) {
		fixture := newPreviewFixture(t, "file:conflict-preview-station-inactive?mode=memory&cache=shared")
		if _, err := fixture.stations.Update(fixture.stationB.ID, fixture.stationB.Version, map[string]any{"station_status": "maintenance"}); err != nil {
			t.Fatal(err)
		}
		reassign := fixture.suggestion(t, "assign_compatible_station")
		_, err := fixture.service.Preview(fixture.pending.ID, dto.ConflictPreviewRequest{ExpectedVersion: fixture.pending.Version, ActionKey: reassign.ActionKey})
		blockers := assertPreviewBlocked(t, err, constants.PreviewBlockerTargetStation, "inactive")
		if blockers[0].ID != fixture.stationB.ID {
			t.Fatalf("expected blocker for station %d, got %+v", fixture.stationB.ID, blockers[0])
		}
	})
	t.Run("alternate window cancelled", func(t *testing.T) {
		fixture := newPreviewFixture(t, "file:conflict-preview-alternate-cancelled?mode=memory&cache=shared")
		if _, err := fixture.windows.Update(fixture.members[2].ID, fixture.members[2].Version, map[string]any{"window_status": constants.WindowStatusCancelled}); err != nil {
			t.Fatal(err)
		}
		alternate := fixture.suggestion(t, "use_alternate_window")
		_, err := fixture.service.Preview(fixture.pending.ID, dto.ConflictPreviewRequest{ExpectedVersion: fixture.pending.Version, ActionKey: alternate.ActionKey})
		blockers := assertPreviewBlocked(t, err, constants.PreviewBlockerAlternateWindow, "cancelled")
		if blockers[0].ID != fixture.members[2].ID {
			t.Fatalf("expected blocker for alternate window %d, got %+v", fixture.members[2].ID, blockers[0])
		}
	})
	t.Run("resolution state and version are guarded", func(t *testing.T) {
		fixture := newPreviewFixture(t, "file:conflict-preview-guards?mode=memory&cache=shared")
		reassign := fixture.suggestion(t, "assign_compatible_station")
		_, err := fixture.service.Preview(fixture.pending.ID, dto.ConflictPreviewRequest{ExpectedVersion: fixture.pending.Version + 9, ActionKey: reassign.ActionKey})
		assertPreviewError(t, err, "version_conflict")
		if _, err := fixture.service.Review(fixture.pending.ID, dto.ConflictActionRequest{ExpectedVersion: fixture.pending.Version, Decision: constants.ResolutionStatusRejected, ReviewNote: "guard check"}, fixture.reviewer, "preview-reject"); err != nil {
			t.Fatal(err)
		}
		_, err = fixture.service.Preview(fixture.pending.ID, dto.ConflictPreviewRequest{ExpectedVersion: fixture.pending.Version + 1, ActionKey: reassign.ActionKey})
		assertPreviewError(t, err, "invalid_state")
	})
}

func assertPreviewBlocked(t *testing.T, err error, kind, reason string) []dto.PreviewBlocker {
	t.Helper()
	var appError *AppError
	if !errors.As(err, &appError) || appError.Status != http.StatusConflict || appError.Code != "preview_blocked" {
		t.Fatalf("expected 409 preview_blocked, got %v", err)
	}
	blockers, ok := appError.Details.([]dto.PreviewBlocker)
	if !ok || len(blockers) == 0 {
		t.Fatalf("expected blocker details, got %+v", appError.Details)
	}
	for _, blocker := range blockers {
		if blocker.Kind == kind && blocker.Reason == reason {
			return blockers
		}
	}
	t.Fatalf("expected a %s/%s blocker, got %+v", kind, reason, blockers)
	return nil
}

func assertPreviewError(t *testing.T, err error, code string) {
	t.Helper()
	var appError *AppError
	if !errors.As(err, &appError) || appError.Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
}
