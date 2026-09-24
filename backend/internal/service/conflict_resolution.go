package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"gorm.io/gorm"

	"satellite-contact-window-deconfliction/backend/internal/config"
	"satellite-contact-window-deconfliction/backend/internal/constants"
	"satellite-contact-window-deconfliction/backend/internal/dto"
	"satellite-contact-window-deconfliction/backend/internal/model"
	"satellite-contact-window-deconfliction/backend/internal/repository"
	"satellite-contact-window-deconfliction/backend/internal/scheduler"
)

type ConflictResolutionService struct {
	repository *repository.ConflictResolutionRepository
	windows    *repository.ContactWindowRepository
	stations   *repository.GroundStationRepository
	assets     *repository.SatelliteAssetRepository
	audit      *AuditService
	weights    scheduler.Weights
}

func NewConflictResolutionService(conflicts *repository.ConflictResolutionRepository, windows *repository.ContactWindowRepository, stations *repository.GroundStationRepository, assets *repository.SatelliteAssetRepository, audit *AuditService, weights config.Weights) *ConflictResolutionService {
	return &ConflictResolutionService{
		repository: conflicts, windows: windows, stations: stations, assets: assets, audit: audit,
		weights: scheduler.Weights{PriorityLoss: weights.PriorityLoss, MovementDistance: weights.MovementDistance, ContactDuration: weights.ContactDuration, ResourceMargin: weights.ResourceMargin},
	}
}

func (service *ConflictResolutionService) List(page, pageSize int, status, conflictType string) ([]dto.ConflictResolutionResponse, dto.PageMeta, error) {
	resolutions, total, err := service.repository.List(page, pageSize, status, conflictType)
	if err != nil {
		return nil, dto.PageMeta{}, Internal("could not list conflict resolutions", err)
	}
	responses := make([]dto.ConflictResolutionResponse, 0, len(resolutions))
	for _, resolution := range resolutions {
		response, parseErr := resolutionResponse(resolution)
		if parseErr != nil {
			return nil, dto.PageMeta{}, parseErr
		}
		responses = append(responses, response)
	}
	return responses, pageMeta(page, pageSize, total), nil
}

func (service *ConflictResolutionService) Get(id uint) (dto.ConflictResolutionResponse, error) {
	resolution, err := service.repository.Get(id)
	if err != nil {
		return dto.ConflictResolutionResponse{}, MapRepositoryError("conflict resolution", err)
	}
	return resolutionResponse(resolution)
}

func (service *ConflictResolutionService) Detect(request dto.DetectConflictsRequest, actor dto.Actor, requestID string) (dto.DetectionResult, error) {
	from, err := time.Parse(time.RFC3339, request.From)
	if err != nil {
		return dto.DetectionResult{}, BadRequest("invalid_time_range", "from must be RFC3339")
	}
	to, err := time.Parse(time.RFC3339, request.To)
	if err != nil {
		return dto.DetectionResult{}, BadRequest("invalid_time_range", "to must be RFC3339")
	}
	if !to.After(from) || to.Sub(from) > 31*24*time.Hour {
		return dto.DetectionResult{}, BadRequest("invalid_time_range", "detection range must be positive and no longer than 31 days")
	}
	windows, err := service.windows.ListRange(from.UTC(), to.UTC())
	if err != nil {
		return dto.DetectionResult{}, Internal("could not load windows for detection", err)
	}
	stations, err := service.stations.ListAll()
	if err != nil {
		return dto.DetectionResult{}, Internal("could not load stations for detection", err)
	}
	assets, err := service.assets.ListAll()
	if err != nil {
		return dto.DetectionResult{}, Internal("could not load satellites for detection", err)
	}
	stationMap := map[uint]model.GroundStation{}
	assetMap := map[uint]model.SatelliteAsset{}
	for _, station := range stations {
		stationMap[station.ID] = station
	}
	for _, asset := range assets {
		assetMap[asset.ID] = asset
	}
	groups := scheduler.Detect(scheduler.DetectionContext{Windows: windows, Stations: stationMap, Satellites: assetMap})
	generator := scheduler.NewCandidateGenerator(service.weights, stations, assets, windows)
	responses := make([]dto.ConflictResolutionResponse, 0, len(groups))
	for _, group := range groups {
		resolution, err := service.persistDetection(group, generator.Generate(group), actor, requestID)
		if err != nil {
			return dto.DetectionResult{}, err
		}
		responses = append(responses, resolution)
	}
	if err := service.audit.Record(actor, requestID, "conflicts.scanned", "planning_range", from.UTC().Format(time.RFC3339), map[string]any{"from": from.UTC(), "to": to.UTC(), "window_count": len(windows), "weights": service.weights}, nil, map[string]any{"conflict_count": len(groups)}); err != nil {
		return dto.DetectionResult{}, err
	}
	return dto.DetectionResult{RangeFrom: from.UTC(), RangeTo: to.UTC(), WindowCount: len(windows), ConflictCount: len(groups), Resolutions: responses}, nil
}

func (service *ConflictResolutionService) persistDetection(group scheduler.ConflictGroup, suggestions []scheduler.Suggestion, actor dto.Actor, requestID string) (dto.ConflictResolutionResponse, error) {
	if existing, err := service.repository.FindByKey(group.Key); err == nil {
		return resolutionResponse(existing)
	}
	windowIDs := make([]uint, 0, len(group.Windows))
	versions := map[string]uint{}
	facts := make([]map[string]any, 0, len(group.Windows))
	for _, window := range group.Windows {
		windowIDs = append(windowIDs, window.ID)
		versions[strconv.FormatUint(uint64(window.ID), 10)] = window.Version
		facts = append(facts, map[string]any{"id": window.ID, "station_id": window.StationID, "satellite_id": window.SatelliteID, "start_at": window.StartAt, "end_at": window.EndAt, "duration_sec": window.DurationSec(), "band": window.Band, "priority": window.Priority, "locked": window.Locked, "version": window.Version})
	}
	evidence := dto.ConflictEvidence{Summary: group.Summary, WindowFacts: facts, Capacity: group.Capacity, PeakConcurrency: group.PeakConcurrency, BufferSeconds: group.BufferSeconds, Metadata: group.Metadata}
	resolution := model.ConflictResolution{
		ConflictKey: group.Key, WindowIDsJSON: mustJSON(windowIDs), WindowVersionsJSON: mustJSON(versions), ConflictType: group.ConflictType,
		EvidenceJSON: mustJSON(evidence), SuggestionsJSON: mustJSON(suggestions), WeightsJSON: mustJSON(service.weights), SelectedAction: "{}",
		ResolutionStatus: constants.ResolutionStatusDetected, Version: 1,
	}
	err := service.repository.DB().Transaction(func(tx *gorm.DB) error {
		txRepository := service.repository.WithDB(tx)
		if err := txRepository.Create(&resolution); err != nil {
			return err
		}
		if err := service.audit.RecordTx(tx, actor, requestID, "conflict.detected", "conflict_resolution", auditID(resolution.ID), map[string]any{"conflict_type": group.ConflictType, "window_ids": windowIDs, "weights": service.weights}, nil, map[string]any{"status": constants.ResolutionStatusDetected, "version": 1}); err != nil {
			return err
		}
		updated, err := txRepository.Transition(tx, resolution.ID, 1, constants.ResolutionStatusDetected, constants.ResolutionStatusProposed, map[string]any{})
		if err != nil {
			return err
		}
		if !updated {
			return errors.New("detected resolution could not enter proposed state")
		}
		return service.audit.RecordTx(tx, actor, requestID, "conflict.proposed", "conflict_resolution", auditID(resolution.ID), map[string]any{"suggestion_count": len(suggestions)}, map[string]any{"status": constants.ResolutionStatusDetected}, map[string]any{"status": constants.ResolutionStatusProposed, "version": 2})
	})
	if err != nil {
		if existing, findErr := service.repository.FindByKey(group.Key); findErr == nil {
			return resolutionResponse(existing)
		}
		return dto.ConflictResolutionResponse{}, Internal("could not persist conflict detection", err)
	}
	created, err := service.repository.Get(resolution.ID)
	if err != nil {
		return dto.ConflictResolutionResponse{}, Internal("could not reload conflict resolution", err)
	}
	return resolutionResponse(created)
}

func (service *ConflictResolutionService) Submit(id uint, request dto.ConflictActionRequest, actor dto.Actor, requestID string) (dto.ConflictResolutionResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.ConflictResolutionResponse{}, MapRepositoryError("conflict resolution", err)
	}
	if !constants.CanTransitionResolution(before.ResolutionStatus, constants.ResolutionStatusPendingReview) {
		return dto.ConflictResolutionResponse{}, Conflict("invalid_state", "only proposed conflicts can be submitted for review", nil)
	}
	updated, err := service.repository.Transition(service.repository.DB(), id, request.ExpectedVersion, constants.ResolutionStatusProposed, constants.ResolutionStatusPendingReview, map[string]any{})
	if err != nil {
		return dto.ConflictResolutionResponse{}, Internal("could not submit conflict resolution", err)
	}
	if !updated {
		return dto.ConflictResolutionResponse{}, Conflict("version_conflict", "conflict resolution changed; reload before submitting", nil)
	}
	after, _ := service.repository.Get(id)
	if err := service.audit.Record(actor, requestID, "conflict.submitted", "conflict_resolution", auditID(id), map[string]any{"expected_version": request.ExpectedVersion}, resolutionSummary(before), resolutionSummary(after)); err != nil {
		return dto.ConflictResolutionResponse{}, err
	}
	return resolutionResponse(after)
}

func (service *ConflictResolutionService) Review(id uint, request dto.ConflictActionRequest, actor dto.Actor, requestID string) (dto.ConflictResolutionResponse, error) {
	if request.Decision != constants.ResolutionStatusAccepted && request.Decision != constants.ResolutionStatusRejected {
		return dto.ConflictResolutionResponse{}, BadRequest("invalid_decision", "decision must be accepted or rejected")
	}
	if request.Decision == constants.ResolutionStatusAccepted && request.ActionKey == "" {
		return dto.ConflictResolutionResponse{}, BadRequest("action_required", "an accepted resolution must select an action")
	}
	err := service.repository.DB().Transaction(func(tx *gorm.DB) error {
		resolution, err := service.repository.GetForUpdate(tx, id)
		if err != nil {
			return MapRepositoryError("conflict resolution", err)
		}
		if resolution.Version != request.ExpectedVersion {
			return Conflict("version_conflict", "conflict resolution changed; reload before reviewing", nil)
		}
		if !constants.CanTransitionResolution(resolution.ResolutionStatus, request.Decision) {
			return Conflict("invalid_state", "only pending review conflicts can be accepted or rejected", nil)
		}
		values := map[string]any{"resolved_by": actor.Username, "review_note": request.ReviewNote, "resolved_at": time.Now().UTC()}
		if request.Decision == constants.ResolutionStatusAccepted {
			selected, err := selectSuggestion(resolution.SuggestionsJSON, request.ActionKey)
			if err != nil {
				return err
			}
			if err := service.verifyWindowVersions(tx, resolution.WindowVersionsJSON); err != nil {
				return err
			}
			values["selected_action"] = mustJSON(selected)
		}
		updated, err := service.repository.Transition(tx, id, request.ExpectedVersion, constants.ResolutionStatusPendingReview, request.Decision, values)
		if err != nil {
			return err
		}
		if !updated {
			return Conflict("version_conflict", "resolution changed during review", nil)
		}
		parameters := map[string]any{"decision": request.Decision, "action_key": request.ActionKey, "review_note_length": len(request.ReviewNote), "expected_version": request.ExpectedVersion}
		return service.audit.RecordTx(tx, actor, requestID, "conflict.reviewed", "conflict_resolution", auditID(id), parameters, resolutionSummary(resolution), map[string]any{"status": request.Decision, "selected_action_key": request.ActionKey, "resolved_by": actor.Username, "version": request.ExpectedVersion + 1})
	})
	if err != nil {
		return dto.ConflictResolutionResponse{}, err
	}
	return service.Get(id)
}

// Preview rehearses the selected plan against current planning data without
// touching windows, audit events, or the review state. Referenced entities
// that changed or disappeared are reported as 409 blockers instead.
func (service *ConflictResolutionService) Preview(id uint, request dto.ConflictPreviewRequest) (dto.ConflictPreviewResponse, error) {
	resolution, err := service.repository.Get(id)
	if err != nil {
		return dto.ConflictPreviewResponse{}, MapRepositoryError("conflict resolution", err)
	}
	if resolution.ResolutionStatus != constants.ResolutionStatusPendingReview {
		return dto.ConflictPreviewResponse{}, Conflict("invalid_state", "only pending review conflicts can be previewed", nil)
	}
	if resolution.Version != request.ExpectedVersion {
		return dto.ConflictPreviewResponse{}, Conflict("version_conflict", "conflict resolution changed; reload before previewing", nil)
	}
	selected, err := selectSuggestion(resolution.SuggestionsJSON, request.ActionKey)
	if err != nil {
		return dto.ConflictPreviewResponse{}, err
	}
	windowIDs := []uint{}
	if err := json.Unmarshal([]byte(resolution.WindowIDsJSON), &windowIDs); err != nil {
		return dto.ConflictPreviewResponse{}, Internal("stored window IDs are invalid", err)
	}
	versions := map[string]uint{}
	if err := json.Unmarshal([]byte(resolution.WindowVersionsJSON), &versions); err != nil {
		return dto.ConflictPreviewResponse{}, Internal("stored window version snapshot is invalid", err)
	}
	windows, err := service.windows.GetMany(windowIDs)
	if err != nil {
		return dto.ConflictPreviewResponse{}, Internal("could not load conflict windows for preview", err)
	}
	current := map[uint]model.ContactWindow{}
	for _, window := range windows {
		current[window.ID] = window
	}
	blockers := make([]dto.PreviewBlocker, 0)
	members := make([]model.ContactWindow, 0, len(windowIDs))
	for _, windowID := range windowIDs {
		window, ok := current[windowID]
		if !ok {
			blockers = append(blockers, dto.PreviewBlocker{Kind: constants.PreviewBlockerWindow, ID: windowID, Reason: "missing", Detail: "conflict window no longer exists"})
			continue
		}
		stored := versions[strconv.FormatUint(uint64(windowID), 10)]
		if stored != window.Version {
			blockers = append(blockers, dto.PreviewBlocker{Kind: constants.PreviewBlockerWindow, ID: windowID, Reason: "changed", Detail: fmt.Sprintf("window is at version %d but the conflict evidence was built from version %d", window.Version, stored)})
		}
		members = append(members, window)
	}
	var affected *model.ContactWindow
	if len(selected.MoveWindowIDs) > 0 {
		if window, ok := current[selected.MoveWindowIDs[0]]; ok {
			moved := window
			affected = &moved
		}
	}
	if selected.TargetStationID != nil {
		station, stationErr := service.stations.Get(*selected.TargetStationID)
		switch {
		case stationErr != nil && isStatus(MapRepositoryError("ground station", stationErr), http.StatusNotFound):
			blockers = append(blockers, dto.PreviewBlocker{Kind: constants.PreviewBlockerTargetStation, ID: *selected.TargetStationID, Reason: "missing", Detail: "target station no longer exists"})
		case stationErr != nil:
			return dto.ConflictPreviewResponse{}, MapRepositoryError("ground station", stationErr)
		case station.StationStatus != "active":
			blockers = append(blockers, dto.PreviewBlocker{Kind: constants.PreviewBlockerTargetStation, ID: station.ID, Reason: "inactive", Detail: fmt.Sprintf("target station status is %s", station.StationStatus)})
		case affected != nil && !containsString(decodeStrings(station.SupportedBandsJSON), affected.Band):
			blockers = append(blockers, dto.PreviewBlocker{Kind: constants.PreviewBlockerTargetStation, ID: station.ID, Reason: "incompatible", Detail: fmt.Sprintf("target station no longer supports band %s", affected.Band)})
		}
	}
	var alternate *model.ContactWindow
	if selected.AlternateWindowID != nil {
		window, windowErr := service.windows.Get(*selected.AlternateWindowID)
		switch {
		case windowErr != nil && isStatus(MapRepositoryError("contact window", windowErr), http.StatusNotFound):
			blockers = append(blockers, dto.PreviewBlocker{Kind: constants.PreviewBlockerAlternateWindow, ID: *selected.AlternateWindowID, Reason: "missing", Detail: "alternate window no longer exists"})
		case windowErr != nil:
			return dto.ConflictPreviewResponse{}, MapRepositoryError("contact window", windowErr)
		case window.WindowStatus == constants.WindowStatusCancelled:
			blockers = append(blockers, dto.PreviewBlocker{Kind: constants.PreviewBlockerAlternateWindow, ID: window.ID, Reason: "cancelled", Detail: "alternate window has been cancelled"})
		case affected != nil && (window.SatelliteID != affected.SatelliteID || window.SourceVersion != affected.SourceVersion):
			blockers = append(blockers, dto.PreviewBlocker{Kind: constants.PreviewBlockerAlternateWindow, ID: window.ID, Reason: "changed", Detail: "alternate window no longer matches the affected window satellite and source version"})
		default:
			candidate := window
			alternate = &candidate
		}
	}
	if len(blockers) > 0 {
		return dto.ConflictPreviewResponse{}, Conflict("preview_blocked", "selected plan references planning data that changed or no longer exists", nil).WithDetails(blockers)
	}
	dispositions := previewDispositions(members, selected)
	simulated, err := service.simulatedWindows(members, selected, alternate)
	if err != nil {
		return dto.ConflictPreviewResponse{}, err
	}
	stations, err := service.stations.ListAll()
	if err != nil {
		return dto.ConflictPreviewResponse{}, Internal("could not load stations for preview", err)
	}
	assets, err := service.assets.ListAll()
	if err != nil {
		return dto.ConflictPreviewResponse{}, Internal("could not load satellites for preview", err)
	}
	stationMap := map[uint]model.GroundStation{}
	assetMap := map[uint]model.SatelliteAsset{}
	for _, station := range stations {
		stationMap[station.ID] = station
	}
	for _, asset := range assets {
		assetMap[asset.ID] = asset
	}
	groups := scheduler.Detect(scheduler.DetectionContext{Windows: simulated, Stations: stationMap, Satellites: assetMap})
	remaining := make([]dto.RemainingConflict, 0, len(groups))
	for _, group := range groups {
		ids := make([]uint, 0, len(group.Windows))
		for _, window := range group.Windows {
			ids = append(ids, window.ID)
		}
		remaining = append(remaining, dto.RemainingConflict{ConflictType: group.ConflictType, Summary: group.Summary, WindowIDs: ids})
	}
	return dto.ConflictPreviewResponse{
		ResolutionID: resolution.ID, ActionKey: selected.ActionKey, ActionType: selected.ActionType, RequiresManual: selected.RequiresManual,
		Dispositions: dispositions, RemainingConflicts: len(remaining), RemainingReasons: remaining, EvaluatedAt: time.Now().UTC(),
	}, nil
}

func previewDispositions(members []model.ContactWindow, selected dto.ResolutionSuggestion) []dto.PreviewDisposition {
	keep := idSet(selected.KeepWindowIDs)
	move := idSet(selected.MoveWindowIDs)
	dispositions := make([]dto.PreviewDisposition, 0, len(members))
	for _, window := range members {
		item := dto.PreviewDisposition{WindowID: window.ID}
		switch {
		case selected.RequiresManual:
			item.Disposition = constants.PreviewDispositionManual
			item.Reason = "selected plan requires a human planning decision; no automatic change is simulated"
		case keep[window.ID]:
			item.Disposition = constants.PreviewDispositionKept
			item.Reason = "plan keeps this window on its current station and interval"
		case move[window.ID] && selected.TargetStationID != nil:
			item.Disposition = constants.PreviewDispositionReassigned
			item.TargetStationID = selected.TargetStationID
			item.Reason = fmt.Sprintf("plan would reassign this window to station #%d", *selected.TargetStationID)
		case move[window.ID] && selected.AlternateWindowID != nil:
			item.Disposition = constants.PreviewDispositionUseAlternate
			item.AlternateWindowID = selected.AlternateWindowID
			item.Reason = fmt.Sprintf("plan would replace this window with alternate window #%d", *selected.AlternateWindowID)
		case move[window.ID]:
			item.Disposition = constants.PreviewDispositionManual
			item.Reason = "plan moves this window but provides no automatic target"
		default:
			item.Disposition = constants.PreviewDispositionKept
			item.Reason = "plan leaves this window on its current station and interval"
		}
		dispositions = append(dispositions, item)
	}
	return dispositions
}

func (service *ConflictResolutionService) simulatedWindows(members []model.ContactWindow, selected dto.ResolutionSuggestion, alternate *model.ContactWindow) ([]model.ContactWindow, error) {
	if len(members) == 0 {
		return nil, Internal("conflict resolution has no windows to simulate", nil)
	}
	from, to := members[0].StartAt, members[0].EndAt
	for _, window := range members[1:] {
		if window.StartAt.Before(from) {
			from = window.StartAt
		}
		if window.EndAt.After(to) {
			to = window.EndAt
		}
	}
	if alternate != nil {
		if alternate.StartAt.Before(from) {
			from = alternate.StartAt
		}
		if alternate.EndAt.After(to) {
			to = alternate.EndAt
		}
	}
	windows, err := service.windows.ListRange(from, to)
	if err != nil {
		return nil, Internal("could not load windows for preview simulation", err)
	}
	if selected.RequiresManual {
		return windows, nil
	}
	reassigned := map[uint]uint{}
	replaced := map[uint]bool{}
	for _, windowID := range selected.MoveWindowIDs {
		if selected.TargetStationID != nil {
			reassigned[windowID] = *selected.TargetStationID
		}
		if selected.AlternateWindowID != nil {
			replaced[windowID] = true
		}
	}
	simulated := make([]model.ContactWindow, 0, len(windows)+1)
	present := map[uint]bool{}
	for _, window := range windows {
		if replaced[window.ID] {
			continue
		}
		if target, ok := reassigned[window.ID]; ok {
			window.StationID = target
		}
		present[window.ID] = true
		simulated = append(simulated, window)
	}
	if alternate != nil && !present[alternate.ID] {
		simulated = append(simulated, *alternate)
	}
	return simulated, nil
}

func idSet(ids []uint) map[uint]bool {
	set := make(map[uint]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

func isStatus(err error, status int) bool {
	var appError *AppError
	return errors.As(err, &appError) && appError.Status == status
}

func (service *ConflictResolutionService) Export(id uint) (map[string]any, error) {
	resolution, err := service.Get(id)
	if err != nil {
		return nil, err
	}
	if resolution.ResolutionStatus != constants.ResolutionStatusAccepted {
		return nil, Conflict("invalid_state", "only accepted resolutions can be exported", nil)
	}
	return map[string]any{
		"record_type": "offline_contact_planning_decision", "resolution_id": resolution.ID, "conflict_key": resolution.ConflictKey,
		"window_ids": resolution.WindowIDs, "selected_action": resolution.SelectedAction, "resolved_by": resolution.ResolvedBy,
		"resolved_at": resolution.ResolvedAt, "disclaimer": "Planning record only; no antenna or spacecraft control command is emitted.",
	}, nil
}

func (service *ConflictResolutionService) verifyWindowVersions(tx *gorm.DB, encoded string) error {
	versions := map[string]uint{}
	if err := json.Unmarshal([]byte(encoded), &versions); err != nil {
		return Internal("stored window version snapshot is invalid", err)
	}
	keys := make([]string, 0, len(versions))
	for id := range versions {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	for _, encodedID := range keys {
		parsed, err := strconv.ParseUint(encodedID, 10, 64)
		if err != nil {
			return Internal("stored window ID is invalid", err)
		}
		window, err := service.windows.FindForUpdate(tx, uint(parsed))
		if err != nil {
			return MapRepositoryError("contact window", err)
		}
		if window.Version != versions[encodedID] {
			return Conflict("version_conflict", fmt.Sprintf("window %d changed after conflict detection", window.ID), nil)
		}
	}
	return nil
}

func resolutionResponse(resolution model.ConflictResolution) (dto.ConflictResolutionResponse, error) {
	response := dto.ConflictResolutionResponse{
		ID: resolution.ID, ConflictKey: resolution.ConflictKey, ConflictType: resolution.ConflictType, ResolutionStatus: resolution.ResolutionStatus,
		ResolvedBy: resolution.ResolvedBy, ReviewNote: resolution.ReviewNote, Version: resolution.Version, ResolvedAt: resolution.ResolvedAt,
		CreatedAt: resolution.CreatedAt, UpdatedAt: resolution.UpdatedAt,
	}
	if err := json.Unmarshal([]byte(resolution.WindowIDsJSON), &response.WindowIDs); err != nil {
		return response, Internal("stored window IDs are invalid", err)
	}
	if err := json.Unmarshal([]byte(resolution.EvidenceJSON), &response.Evidence); err != nil {
		return response, Internal("stored conflict evidence is invalid", err)
	}
	if err := json.Unmarshal([]byte(resolution.SuggestionsJSON), &response.Suggestions); err != nil {
		return response, Internal("stored suggestions are invalid", err)
	}
	if resolution.SelectedAction != "" && resolution.SelectedAction != "{}" {
		selected := dto.ResolutionSuggestion{}
		if err := json.Unmarshal([]byte(resolution.SelectedAction), &selected); err != nil {
			return response, Internal("stored selected action is invalid", err)
		}
		response.SelectedAction = &selected
	}
	return response, nil
}

func selectSuggestion(encoded, key string) (dto.ResolutionSuggestion, error) {
	var suggestions []dto.ResolutionSuggestion
	if err := json.Unmarshal([]byte(encoded), &suggestions); err != nil {
		return dto.ResolutionSuggestion{}, Internal("stored suggestions are invalid", err)
	}
	for _, suggestion := range suggestions {
		if suggestion.ActionKey == key {
			return suggestion, nil
		}
	}
	return dto.ResolutionSuggestion{}, BadRequest("unknown_action", "selected action is not part of this resolution")
}

func mustJSON(value any) string { encoded, _ := json.Marshal(value); return string(encoded) }
func resolutionSummary(resolution model.ConflictResolution) map[string]any {
	return map[string]any{"conflict_type": resolution.ConflictType, "status": resolution.ResolutionStatus, "version": resolution.Version, "resolved_by": resolution.ResolvedBy}
}
