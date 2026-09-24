package service

import (
	"errors"
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
	db        *gorm.DB
	service   *ConflictResolutionService
	audit     *AuditService
	windows   []model.ContactWindow
	stations  []model.GroundStation
	assets    []model.SatelliteAsset
	resolved  dto.ConflictResolutionResponse
	scheduler dto.Actor
	reviewer  dto.Actor
}

func newPreviewFixture(t *testing.T, dsn string, extraBuilders ...func(base time.Time, stations []model.GroundStation, assets []model.SatelliteAsset) []model.ContactWindow) previewFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.GroundStation{}, &model.SatelliteAsset{}, &model.ContactWindow{}, &model.ConflictResolution{}, &model.AuditEvent{}); err != nil {
		t.Fatal(err)
	}
	stations := []model.GroundStation{
		{StationCode: "PV-GS1", Name: "Primary", AntennaCount: 1, SupportedBandsJSON: `["S"]`, SlewBufferSec: 0, StationStatus: "active", Version: 1},
		{StationCode: "PV-GS2", Name: "Secondary", Latitude: 1, Longitude: 1, AntennaCount: 2, SupportedBandsJSON: `["S"]`, SlewBufferSec: 0, StationStatus: "active", Version: 1},
	}
	assets := []model.SatelliteAsset{
		{SatelliteCode: "PV-A", Name: "A", SupportedBandsJSON: `["S"]`, MinimumContactSec: 60, AssetStatus: "active", PriorityWeight: 2, Version: 1},
		{SatelliteCode: "PV-B", Name: "B", SupportedBandsJSON: `["S"]`, MinimumContactSec: 60, AssetStatus: "active", PriorityWeight: 1, Version: 1},
	}
	if err := db.Create(&stations).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&assets).Error; err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Truncate(time.Second)
	windows := []model.ContactWindow{
		{StationID: stations[0].ID, SatelliteID: assets[0].ID, StartAt: base, EndAt: base.Add(10 * time.Minute), Band: "S", WindowStatus: constants.WindowStatusSubmitted, Priority: 8, SourceVersion: "pv-source", Version: 1},
		{StationID: stations[0].ID, SatelliteID: assets[1].ID, StartAt: base.Add(time.Minute), EndAt: base.Add(9 * time.Minute), Band: "S", WindowStatus: constants.WindowStatusSubmitted, Priority: 5, SourceVersion: "pv-source", Version: 1},
	}
	if err := db.Create(&windows).Error; err != nil {
		t.Fatal(err)
	}
	for _, build := range extraBuilders {
		extra := build(base, stations, assets)
		if err := db.Create(&extra).Error; err != nil {
			t.Fatal(err)
		}
		windows = append(windows, extra...)
	}
	stationRepository := repository.NewGroundStationRepository(db)
	assetRepository := repository.NewSatelliteAssetRepository(db)
	windowRepository := repository.NewContactWindowRepository(db)
	conflictRepository := repository.NewConflictResolutionRepository(db)
	systemRepository := repository.NewSystemRepository(db)
	audit := NewAuditService(systemRepository)
	resolutionService := NewConflictResolutionService(conflictRepository, windowRepository, stationRepository, assetRepository, audit, config.Weights{PriorityLoss: 4, MovementDistance: .02, ContactDuration: .003, ResourceMargin: 2})
	schedulerActor := dto.Actor{ID: 1, Username: "scheduler", Role: constants.RoleScheduler}
	reviewerActor := dto.Actor{ID: 2, Username: "reviewer", Role: constants.RoleReviewer}
	detectTo := base.Add(time.Hour)
	if len(extraBuilders) > 0 {
		detectTo = base.Add(4 * time.Hour)
	}
	detected, err := resolutionService.Detect(dto.DetectConflictsRequest{From: base.Add(-time.Minute).Format(time.RFC3339), To: detectTo.Format(time.RFC3339)}, schedulerActor, "pv-detect")
	if err != nil {
		t.Fatal(err)
	}
	var resolved dto.ConflictResolutionResponse
	for _, item := range detected.Resolutions {
		if item.ConflictType == constants.ConflictTypeStationCapacity {
			resolved = item
			break
		}
	}
	if resolved.ID == 0 {
		t.Fatal("expected a station capacity conflict")
	}
	resolved, err = resolutionService.Submit(resolved.ID, dto.ConflictActionRequest{ExpectedVersion: resolved.Version}, schedulerActor, "pv-submit")
	if err != nil {
		t.Fatal(err)
	}
	return previewFixture{db: db, service: resolutionService, audit: audit, windows: windows, stations: stations, assets: assets, resolved: resolved, scheduler: schedulerActor, reviewer: reviewerActor}
}

func suggestionByType(resolution dto.ConflictResolutionResponse, actionType string) dto.ResolutionSuggestion {
	for _, suggestion := range resolution.Suggestions {
		if suggestion.ActionType == actionType {
			return suggestion
		}
	}
	return dto.ResolutionSuggestion{}
}

func TestPreviewKeepHighPriorityIsReadonlyAndRemovesConflict(t *testing.T) {
	fixture := newPreviewFixture(t, "file:preview-keep?mode=memory&cache=shared")
	before, err := fixture.service.Get(fixture.resolved.ID)
	if err != nil {
		t.Fatal(err)
	}
	selected := suggestionByType(before, "keep_high_priority")
	if selected.ActionKey == "" {
		t.Fatal("expected a keep_high_priority suggestion")
	}
	result, err := fixture.service.Preview(before.ID, dto.ConflictPreviewRequest{ExpectedVersion: before.Version, ActionKey: selected.ActionKey})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Readonly || result.RemainingCount != 0 || len(result.RemainingConflicts) != 0 {
		t.Fatalf("unexpected preview result: %+v", result)
	}
	dispositions := map[uint]string{}
	for _, item := range result.WindowDispositions {
		dispositions[item.WindowID] = item.Disposition
	}
	if dispositions[fixture.windows[0].ID] != dto.PreviewDispositionKeep {
		t.Fatalf("high priority window should be kept, got %v", dispositions)
	}
	if dispositions[fixture.windows[1].ID] != dto.PreviewDispositionManual {
		t.Fatalf("lower priority window needs manual handling, got %v", dispositions)
	}

	// Nothing was written: resolution, windows, and audit trail are untouched.
	after, err := fixture.service.Get(before.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Version != before.Version || after.ResolutionStatus != constants.ResolutionStatusPendingReview {
		t.Fatalf("preview mutated the resolution: %+v", after)
	}
	var stored model.ContactWindow
	if err := fixture.db.First(&stored, fixture.windows[1].ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.StationID != fixture.windows[1].StationID || stored.Version != 1 {
		t.Fatalf("preview mutated the moved window: %+v", stored)
	}
	events, _, err := fixture.audit.List(1, 100, "conflict_resolution", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.Action == "conflict.previewed" {
			t.Fatal("preview must not record audit events")
		}
	}
}

func TestPreviewAssignsCompatibleStationWithZeroRemainingConflicts(t *testing.T) {
	fixture := newPreviewFixture(t, "file:preview-reassign?mode=memory&cache=shared")
	resolution, err := fixture.service.Get(fixture.resolved.ID)
	if err != nil {
		t.Fatal(err)
	}
	selected := suggestionByType(resolution, "assign_compatible_station")
	if selected.ActionKey == "" || selected.TargetStationID == nil || *selected.TargetStationID != fixture.stations[1].ID {
		t.Fatalf("expected reassignment to secondary station, got %+v", selected)
	}
	result, err := fixture.service.Preview(resolution.ID, dto.ConflictPreviewRequest{ExpectedVersion: resolution.Version, ActionKey: selected.ActionKey})
	if err != nil {
		t.Fatal(err)
	}
	if result.RemainingCount != 0 {
		t.Fatalf("expected no remaining conflicts, got %+v", result.RemainingConflicts)
	}
	reassigned := 0
	for _, item := range result.WindowDispositions {
		switch item.Disposition {
		case dto.PreviewDispositionReassign:
			reassigned++
			if item.TargetStationID != fixture.stations[1].ID || item.TargetStationCode != "PV-GS2" {
				t.Fatalf("window should be reassigned to PV-GS2, got %+v", item)
			}
		case dto.PreviewDispositionKeep:
		default:
			t.Fatalf("reassignment only keeps or reassigns windows, got %+v", item)
		}
	}
	if reassigned != 1 {
		t.Fatalf("expected exactly one reassigned window, got %d", reassigned)
	}
	covered := map[uint]bool{}
	for _, item := range result.WindowDispositions {
		covered[item.WindowID] = true
	}
	for _, window := range fixture.windows[:2] {
		if !covered[window.ID] {
			t.Fatalf("window %d missing from per-window dispositions", window.ID)
		}
	}
}

func TestPreviewReturns409BlockersWhenWindowVersionChanges(t *testing.T) {
	fixture := newPreviewFixture(t, "file:preview-blockers?mode=memory&cache=shared")
	resolution, err := fixture.service.Get(fixture.resolved.ID)
	if err != nil {
		t.Fatal(err)
	}
	selected := suggestionByType(resolution, "keep_high_priority")
	updated, err := repository.NewContactWindowRepository(fixture.db).Update(fixture.windows[0].ID, 1, map[string]any{"priority": 9})
	if err != nil || !updated {
		t.Fatalf("could not change window version: %v %v", updated, err)
	}
	_, err = fixture.service.Preview(resolution.ID, dto.ConflictPreviewRequest{ExpectedVersion: resolution.Version, ActionKey: selected.ActionKey})
	var appError *AppError
	if !errors.As(err, &appError) || appError.Status != 409 || appError.Code != previewBlockedCode {
		t.Fatalf("expected preview_blocked 409, got %v", err)
	}
	details, ok := appError.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected blocker details, got %#v", appError.Details)
	}
	blockers, _ := details["blockers"].([]dto.PreviewBlocker)
	found := false
	for _, blocker := range blockers {
		if blocker.Code == "window_version_changed" && blocker.WindowID == fixture.windows[0].ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected window_version_changed blocker, got %#v", blockers)
	}
}

func TestPreviewRejectsNonPendingResolutionAndStaleVersion(t *testing.T) {
	fixture := newPreviewFixture(t, "file:preview-state?mode=memory&cache=shared")
	resolution, err := fixture.service.Get(fixture.resolved.ID)
	if err != nil {
		t.Fatal(err)
	}
	selected := suggestionByType(resolution, "keep_high_priority")
	_, err = fixture.service.Preview(resolution.ID, dto.ConflictPreviewRequest{ExpectedVersion: resolution.Version + 1, ActionKey: selected.ActionKey})
	assertConflictCode(t, err, "version_conflict")

	// Only pending review resolutions can be previewed; a rejected one returns invalid_state.
	if _, err := fixture.service.Review(resolution.ID, dto.ConflictActionRequest{ExpectedVersion: resolution.Version, Decision: constants.ResolutionStatusRejected}, fixture.reviewer, "pv-reject"); err != nil {
		t.Fatal(err)
	}
	_, err = fixture.service.Preview(resolution.ID, dto.ConflictPreviewRequest{ExpectedVersion: resolution.Version + 1, ActionKey: selected.ActionKey})
	assertConflictCode(t, err, "invalid_state")
}

func assertConflictCode(t *testing.T, err error, code string) {
	t.Helper()
	var appError *AppError
	if !errors.As(err, &appError) || appError.Status != 409 || appError.Code != code {
		t.Fatalf("expected %s 409, got %v", code, err)
	}
}

func TestPreviewUsesAlternateWindowWhenSourceMatchedOptionExists(t *testing.T) {
	var alternateID uint
	fixture := newPreviewFixture(t, "file:preview-alternate?mode=memory&cache=shared", func(base time.Time, stations []model.GroundStation, assets []model.SatelliteAsset) []model.ContactWindow {
		alternate := model.ContactWindow{
			StationID: stations[0].ID, SatelliteID: assets[1].ID,
			StartAt: base.Add(90 * time.Minute), EndAt: base.Add(100 * time.Minute), Band: "S",
			WindowStatus: constants.WindowStatusSubmitted, Priority: 5, SourceVersion: "pv-source", Version: 1,
		}
		return []model.ContactWindow{alternate}
	})
	alternateID = fixture.windows[len(fixture.windows)-1].ID
	resolution, err := fixture.service.Get(fixture.resolved.ID)
	if err != nil {
		t.Fatal(err)
	}
	selected := suggestionByType(resolution, "use_alternate_window")
	if selected.ActionKey == "" || selected.AlternateWindowID == nil || *selected.AlternateWindowID != alternateID {
		t.Fatalf("expected suggestion to reference alternate window %d, got %+v", alternateID, selected)
	}
	result, err := fixture.service.Preview(resolution.ID, dto.ConflictPreviewRequest{ExpectedVersion: resolution.Version, ActionKey: selected.ActionKey})
	if err != nil {
		t.Fatal(err)
	}
	if result.RemainingCount != 0 {
		t.Fatalf("expected no remaining conflicts, got %+v", result.RemainingConflicts)
	}
	hasAlternateDisposition := false
	for _, item := range result.WindowDispositions {
		if item.Disposition == dto.PreviewDispositionAlternate && item.AlternateWindowID == alternateID {
			hasAlternateDisposition = true
		}
	}
	if !hasAlternateDisposition {
		t.Fatalf("expected a use_alternate_window disposition, got %+v", result.WindowDispositions)
	}
}

func TestPreviewBlocksWhenAlternateWindowIsCancelled(t *testing.T) {
	var alternateID uint
	fixture := newPreviewFixture(t, "file:preview-alt-cancel?mode=memory&cache=shared", func(base time.Time, stations []model.GroundStation, assets []model.SatelliteAsset) []model.ContactWindow {
		alternate := model.ContactWindow{
			StationID: stations[0].ID, SatelliteID: assets[1].ID,
			StartAt: base.Add(2 * time.Hour), EndAt: base.Add(130 * time.Minute), Band: "S",
			WindowStatus: constants.WindowStatusSubmitted, Priority: 5, SourceVersion: "pv-source", Version: 1,
		}
		return []model.ContactWindow{alternate}
	})
	alternateID = fixture.windows[len(fixture.windows)-1].ID
	resolution, err := fixture.service.Get(fixture.resolved.ID)
	if err != nil {
		t.Fatal(err)
	}
	selected := suggestionByType(resolution, "use_alternate_window")
	if selected.ActionKey == "" {
		t.Fatal("expected use_alternate_window suggestion")
	}
	if updated, err := repository.NewContactWindowRepository(fixture.db).Update(alternateID, 1, map[string]any{"window_status": constants.WindowStatusCancelled}); err != nil || !updated {
		t.Fatalf("could not cancel alternate: %v %v", updated, err)
	}
	_, err = fixture.service.Preview(resolution.ID, dto.ConflictPreviewRequest{ExpectedVersion: resolution.Version, ActionKey: selected.ActionKey})
	var appError *AppError
	if !errors.As(err, &appError) || appError.Code != previewBlockedCode {
		t.Fatalf("expected preview_blocked, got %v", err)
	}
	details, _ := appError.Details.(map[string]any)
	blockers, _ := details["blockers"].([]dto.PreviewBlocker)
	found := false
	for _, blocker := range blockers {
		if blocker.Code == "window_cancelled" && blocker.WindowID == alternateID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected window_cancelled blocker for alternate, got %#v", blockers)
	}
}

func TestPreviewIncludesConflictsOutsideOriginalGroup(t *testing.T) {
	// GS1 (capacity 1) holds the conflicting pair. The unrelated same-satellite
	// window lives on GS2 and already forms a separate satellite_overlap group;
	// the preview must report cross-group residual conflicts, not just the
	// station group being previewed.
	fixture := newPreviewFixture(t, "file:preview-cascade?mode=memory&cache=shared", func(base time.Time, stations []model.GroundStation, assets []model.SatelliteAsset) []model.ContactWindow {
		return []model.ContactWindow{{
			StationID: stations[1].ID, SatelliteID: assets[0].ID,
			StartAt: base.Add(2 * time.Minute), EndAt: base.Add(8 * time.Minute), Band: "S",
			WindowStatus: constants.WindowStatusSubmitted, Priority: 7, SourceVersion: "pv-source", Version: 1,
		}}
	})
	// GS2 keeps capacity 2, so no station-capacity blocker fires; the unrelated
	// window belongs to the same satellite as the moved window, surfacing in a
	// separate satellite_overlap group that must remain visible in the preview.

	// The suggestion was generated against capacity 2, so it still targets GS2;
	// preview must recompute against current data and either block or report the fallout.
	resolution, err := fixture.service.Get(fixture.resolved.ID)
	if err != nil {
		t.Fatal(err)
	}
	selected := suggestionByType(resolution, "assign_compatible_station")
	if selected.ActionKey == "" {
		t.Skip("no station reassignment suggestion available in this fixture")
	}
	result, err := fixture.service.Preview(resolution.ID, dto.ConflictPreviewRequest{ExpectedVersion: resolution.Version, ActionKey: selected.ActionKey})
	if err != nil {
		t.Fatalf("preview should succeed and report the satellite overlap: %v", err)
	}
	found := false
	for _, conflict := range result.RemainingConflicts {
		if conflict.ConflictType == constants.ConflictTypeSatelliteOverlap {
			found = true
		}
	}
	if result.RemainingCount == 0 || !found {
		t.Fatalf("expected the cross-group satellite overlap to remain listed, got %+v", result.RemainingConflicts)
	}
}
