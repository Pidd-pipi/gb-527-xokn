package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"satellite-contact-window-deconfliction/backend/internal/constants"
	"satellite-contact-window-deconfliction/backend/internal/dto"
	"satellite-contact-window-deconfliction/backend/internal/model"
	"satellite-contact-window-deconfliction/backend/internal/scheduler"
)

const previewBlockedCode = "preview_blocked"

// Preview evaluates a pending-review suggestion against the current window
// versions without writing anything: no window, audit, or review state is
// modified. It is the read-only counterpart of the accept path.
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

	stations, err := service.stations.ListAll()
	if err != nil {
		return dto.ConflictPreviewResponse{}, Internal("could not load stations for preview", err)
	}
	assets, err := service.assets.ListAll()
	if err != nil {
		return dto.ConflictPreviewResponse{}, Internal("could not load satellites for preview", err)
	}
	allWindows, referencedIDs, err := service.loadPreviewWindows(resolution, selected)
	if err != nil {
		return dto.ConflictPreviewResponse{}, err
	}
	blockers := service.collectPreviewBlockers(resolution, selected, allWindows, referencedIDs, stations, assets)
	if len(blockers) > 0 {
		return dto.ConflictPreviewResponse{}, ConflictWithDetails(previewBlockedCode, "suggestion references outdated or missing planning data", map[string]any{"blockers": blockers})
	}

	dispositions := buildWindowDispositions(resolution, selected, allWindows, stations)
	simulated := simulateAppliedWindows(selected, allWindows)
	remaining := detectRemainingConflicts(simulated, stations, assets, referencedIDs)

	return dto.ConflictPreviewResponse{
		ResolutionID: resolution.ID, ConflictType: resolution.ConflictType,
		ActionKey: selected.ActionKey, ActionType: selected.ActionType, RequiresManual: selected.RequiresManual,
		WindowDispositions: dispositions, RemainingConflicts: remaining, RemainingCount: len(remaining), Readonly: true,
	}, nil
}

// loadPreviewWindows fetches every non-cancelled window (the same population
// conflict detection uses) and returns it together with the deduplicated set of
// IDs the suggestion or the conflict group references.
func (service *ConflictResolutionService) loadPreviewWindows(resolution model.ConflictResolution, selected dto.ResolutionSuggestion) ([]model.ContactWindow, map[uint]bool, error) {
	var groupIDs []uint
	if err := json.Unmarshal([]byte(resolution.WindowIDsJSON), &groupIDs); err != nil {
		return nil, nil, Internal("stored window IDs are invalid", err)
	}
	referenced := map[uint]bool{}
	for _, id := range groupIDs {
		referenced[id] = true
	}
	for _, id := range selected.KeepWindowIDs {
		referenced[id] = true
	}
	for _, id := range selected.MoveWindowIDs {
		referenced[id] = true
	}
	if selected.AlternateWindowID != nil {
		referenced[*selected.AlternateWindowID] = true
	}
	windows, err := service.windows.ListAll()
	if err != nil {
		return nil, nil, Internal("could not load windows for preview", err)
	}
	return windows, referenced, nil
}

func (service *ConflictResolutionService) collectPreviewBlockers(resolution model.ConflictResolution, selected dto.ResolutionSuggestion, allWindows []model.ContactWindow, referenced map[uint]bool, stations []model.GroundStation, assets []model.SatelliteAsset) []dto.PreviewBlocker {
	versions := map[string]uint{}
	if err := json.Unmarshal([]byte(resolution.WindowVersionsJSON), &versions); err != nil {
		versions = map[string]uint{}
	}
	windowByID := map[uint]model.ContactWindow{}
	for _, window := range allWindows {
		windowByID[window.ID] = window
	}
	stationByID := map[uint]model.GroundStation{}
	for _, station := range stations {
		stationByID[station.ID] = station
	}
	assetByID := map[uint]model.SatelliteAsset{}
	for _, asset := range assets {
		assetByID[asset.ID] = asset
	}

	blockers := []dto.PreviewBlocker{}
	add := func(blocker dto.PreviewBlocker) { blockers = append(blockers, blocker) }
	groupIDs := make([]uint, 0)
	for id := range referenced {
		if _, tracked := versions[strconv.FormatUint(uint64(id), 10)]; tracked {
			groupIDs = append(groupIDs, id)
		}
	}

	for _, id := range sortedWindowIDs(referenced) {
		window, exists := windowByID[id]
		expectedRaw, tracked := versions[strconv.FormatUint(uint64(id), 10)]
		if !exists {
			blocker := dto.PreviewBlocker{Code: "window_not_found", Message: fmt.Sprintf("window %d no longer exists", id), WindowID: id}
			if tracked {
				expected := expectedRaw
				blocker.ExpectedVersion = &expected
			}
			add(blocker)
			continue
		}
		if tracked && window.Version != expectedRaw {
			add(dto.PreviewBlocker{Code: "window_version_changed", Message: fmt.Sprintf("window %d changed after conflict detection (v%d -> v%d)", id, expectedRaw, window.Version), WindowID: id, ExpectedVersion: uintPtr(expectedRaw), CurrentVersion: uintPtr(window.Version)})
		}
		if window.WindowStatus == constants.WindowStatusCancelled {
			add(dto.PreviewBlocker{Code: "window_cancelled", Message: fmt.Sprintf("window %d was cancelled", id), WindowID: id})
		}
		if containsWindowID(selected.MoveWindowIDs, id) && window.Locked {
			add(dto.PreviewBlocker{Code: "window_locked", Message: fmt.Sprintf("window %d is locked and cannot be moved", id), WindowID: id})
		}
	}

	if selected.TargetStationID != nil {
		target, exists := stationByID[*selected.TargetStationID]
		if !exists {
			add(dto.PreviewBlocker{Code: "target_station_not_found", Message: fmt.Sprintf("target station %d no longer exists", *selected.TargetStationID), StationID: *selected.TargetStationID})
		} else {
			if target.StationStatus != "active" {
				add(dto.PreviewBlocker{Code: "target_station_inactive", Message: fmt.Sprintf("target station %s is no longer active", target.StationCode), StationID: target.ID})
			}
			for _, movedID := range selected.MoveWindowIDs {
				moved, ok := windowByID[movedID]
				if !ok {
					continue
				}
				if !scheduler.ContainsBand(target.SupportedBandsJSON, moved.Band) {
					add(dto.PreviewBlocker{Code: "target_station_band_incompatible", Message: fmt.Sprintf("station %s no longer supports band %s for window %d", target.StationCode, moved.Band, movedID), StationID: target.ID, WindowID: movedID})
					continue
				}
				generator := scheduler.NewCandidateGenerator(service.weights, stations, assets, allWindows)
				if generator.ConcurrentAt(target.ID, moved) >= target.AntennaCount {
					add(dto.PreviewBlocker{Code: "target_station_capacity_exhausted", Message: fmt.Sprintf("station %s has no free antenna channel for window %d", target.StationCode, movedID), StationID: target.ID, WindowID: movedID})
				}
			}
		}
	}

	if selected.AlternateWindowID != nil {
		alternateID := *selected.AlternateWindowID
		alternate, exists := windowByID[alternateID]
		if !exists {
			add(dto.PreviewBlocker{Code: "alternate_window_not_found", Message: fmt.Sprintf("alternate window %d no longer exists", alternateID), AlternateWindowID: alternateID})
		} else {
			affectedID := uint(0)
			if len(selected.MoveWindowIDs) > 0 {
				affectedID = selected.MoveWindowIDs[0]
			}
			affected, affectedOK := windowByID[affectedID]
			if affectedOK && (alternate.SatelliteID != affected.SatelliteID || alternate.SourceVersion != affected.SourceVersion) {
				add(dto.PreviewBlocker{Code: "alternate_window_source_changed", Message: fmt.Sprintf("alternate window %d no longer matches the source version of window %d", alternateID, affectedID), WindowID: affectedID, AlternateWindowID: alternateID})
			}
			if station, ok := stationByID[alternate.StationID]; !ok || station.StationStatus != "active" || !scheduler.ContainsBand(station.SupportedBandsJSON, alternate.Band) {
				add(dto.PreviewBlocker{Code: "alternate_window_unavailable", Message: fmt.Sprintf("alternate window %d lost station or band compatibility", alternateID), AlternateWindowID: alternateID})
			} else if asset, ok := assetByID[alternate.SatelliteID]; !ok || !scheduler.ContainsBand(asset.SupportedBandsJSON, alternate.Band) {
				add(dto.PreviewBlocker{Code: "alternate_window_unavailable", Message: fmt.Sprintf("alternate window %d lost satellite band compatibility", alternateID), AlternateWindowID: alternateID})
			} else if affectedOK {
				generator := scheduler.NewCandidateGenerator(service.weights, stations, assets, allWindows)
				if !generator.AlternateAvailable(alternate, affected, groupIDs) {
					add(dto.PreviewBlocker{Code: "alternate_window_unavailable", Message: fmt.Sprintf("alternate window %d now overlaps another committed window", alternateID), AlternateWindowID: alternateID})
				}
			}
		}
	}

	sort.SliceStable(blockers, func(i, j int) bool {
		if blockers[i].Code != blockers[j].Code {
			return blockers[i].Code < blockers[j].Code
		}
		return blockers[i].WindowID < blockers[j].WindowID
	})
	return blockers
}

func buildWindowDispositions(resolution model.ConflictResolution, selected dto.ResolutionSuggestion, windows []model.ContactWindow, stations []model.GroundStation) []dto.PreviewWindowDisposition {
	var groupIDs []uint
	if err := json.Unmarshal([]byte(resolution.WindowIDsJSON), &groupIDs); err != nil {
		groupIDs = selected.KeepWindowIDs
	}
	sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i] < groupIDs[j] })
	stationCodes := map[uint]string{}
	for _, station := range stations {
		stationCodes[station.ID] = station.StationCode
	}
	byID := map[uint]model.ContactWindow{}
	for _, window := range windows {
		byID[window.ID] = window
	}
	dispositions := make([]dto.PreviewWindowDisposition, 0)
	noteKept := "Unaffected window keeps its station and interval."
	switch selected.ActionType {
	case "keep_high_priority":
		for _, id := range groupIDs {
			entry := dto.PreviewWindowDisposition{WindowID: id}
			if containsWindowID(selected.KeepWindowIDs, id) {
				entry.Disposition = dto.PreviewDispositionKeep
				entry.DispositionLabel = "retain on the current plan"
				if window, ok := byID[id]; ok && window.Locked {
					entry.Note = "Locked window is retained; automated movement is prohibited."
				} else {
					entry.Note = "Highest ranked window keeps its station and interval."
				}
			} else {
				entry.Disposition = dto.PreviewDispositionManual
				entry.DispositionLabel = "needs manual handling"
				entry.Note = "Removed from this station interval; the planner must reschedule or cancel it."
			}
			dispositions = append(dispositions, entry)
		}
	case "assign_compatible_station":
		for _, id := range groupIDs {
			entry := dto.PreviewWindowDisposition{WindowID: id}
			if containsWindowID(selected.MoveWindowIDs, id) {
				entry.Disposition = dto.PreviewDispositionReassign
				entry.DispositionLabel = "reassign to compatible station"
				if selected.TargetStationID != nil {
					entry.TargetStationID = *selected.TargetStationID
					entry.TargetStationCode = stationCodes[*selected.TargetStationID]
				}
				entry.Note = "Same interval re-evaluated on the nearest compatible station."
			} else {
				entry.Disposition = dto.PreviewDispositionKeep
				entry.DispositionLabel = "retain on the current plan"
				entry.Note = noteKept
			}
			dispositions = append(dispositions, entry)
		}
	case "use_alternate_window":
		alternateID := uint(0)
		if selected.AlternateWindowID != nil {
			alternateID = *selected.AlternateWindowID
		}
		for _, id := range groupIDs {
			entry := dto.PreviewWindowDisposition{WindowID: id}
			if containsWindowID(selected.MoveWindowIDs, id) {
				entry.Disposition = dto.PreviewDispositionAlternate
				entry.DispositionLabel = "use alternate window"
				entry.AlternateWindowID = alternateID
				entry.Note = fmt.Sprintf("Replaced in the plan by source-matched window #%d.", alternateID)
			} else {
				entry.Disposition = dto.PreviewDispositionKeep
				entry.DispositionLabel = "retain on the current plan"
				entry.Note = noteKept
			}
			dispositions = append(dispositions, entry)
		}
		if alternateID != 0 && !containsWindowID(groupIDs, alternateID) {
			dispositions = append(dispositions, dto.PreviewWindowDisposition{WindowID: alternateID, Disposition: dto.PreviewDispositionKeep, DispositionLabel: "retain on the current plan", Note: "Source-matched alternate window takes the contact slot."})
		}
	default: // manual_review and any future action type preserve everything.
		for _, id := range groupIDs {
			dispositions = append(dispositions, dto.PreviewWindowDisposition{WindowID: id, Disposition: dto.PreviewDispositionManual, DispositionLabel: "needs manual handling", Note: "Every window is preserved for an operator-authored decision."})
		}
	}
	return dispositions
}

// simulateAppliedWindows builds the in-memory window set that would remain
// immediately after the suggestion is honoured. Cancelled windows are excluded
// to match the conflict detection input. Nothing is persisted.
func simulateAppliedWindows(selected dto.ResolutionSuggestion, windows []model.ContactWindow) []model.ContactWindow {
	simulated := make([]model.ContactWindow, 0, len(windows))
	moveSet := map[uint]bool{}
	for _, id := range selected.MoveWindowIDs {
		moveSet[id] = true
	}
	for _, window := range windows {
		if window.WindowStatus == constants.WindowStatusCancelled {
			continue
		}
		switch selected.ActionType {
		case "keep_high_priority":
			// Moved windows leave the station timeline and wait for manual planning.
			if moveSet[window.ID] {
				continue
			}
		case "assign_compatible_station":
			if moveSet[window.ID] && selected.TargetStationID != nil {
				window.StationID = *selected.TargetStationID
			}
		case "use_alternate_window":
			// The conflicting window is withdrawn in favour of the source-matched one.
			if moveSet[window.ID] {
				continue
			}
		}
		simulated = append(simulated, window)
	}
	return simulated
}

func detectRemainingConflicts(windows []model.ContactWindow, stations []model.GroundStation, assets []model.SatelliteAsset, relevant map[uint]bool) []dto.PreviewRemainingConflict {
	stationMap := map[uint]model.GroundStation{}
	assetMap := map[uint]model.SatelliteAsset{}
	for _, station := range stations {
		stationMap[station.ID] = station
	}
	for _, asset := range assets {
		assetMap[asset.ID] = asset
	}
	groups := scheduler.Detect(scheduler.DetectionContext{Windows: windows, Stations: stationMap, Satellites: assetMap})
	result := make([]dto.PreviewRemainingConflict, 0)
	for _, group := range groups {
		touches := false
		ids := make([]uint, 0, len(group.Windows))
		for _, window := range group.Windows {
			ids = append(ids, window.ID)
			if relevant[window.ID] {
				touches = true
			}
		}
		if !touches {
			continue
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		result = append(result, dto.PreviewRemainingConflict{ConflictType: group.ConflictType, WindowIDs: ids, Summary: group.Summary})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].ConflictType != result[j].ConflictType {
			return result[i].ConflictType < result[j].ConflictType
		}
		return fmt.Sprint(result[i].WindowIDs) < fmt.Sprint(result[j].WindowIDs)
	})
	return result
}

func sortedWindowIDs(values map[uint]bool) []uint {
	ids := make([]uint, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func containsWindowID(ids []uint, target uint) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func uintPtr(value uint) *uint { return &value }
