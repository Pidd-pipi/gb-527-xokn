package scheduler

import (
	"testing"
	"time"

	"satellite-contact-window-deconfliction/backend/internal/constants"
	"satellite-contact-window-deconfliction/backend/internal/model"
)

func TestDetectCoversConflictFamiliesDeterministically(t *testing.T) {
	base := time.Date(2026, 8, 22, 11, 0, 0, 0, time.UTC)
	station := model.GroundStation{ID: 1, StationCode: "GS-1", AntennaCount: 1, SupportedBandsJSON: `["S"]`, SlewBufferSec: 120}
	assets := map[uint]model.SatelliteAsset{
		1: {ID: 1, SatelliteCode: "SAT-1", SupportedBandsJSON: `["S"]`, MinimumContactSec: 300},
		2: {ID: 2, SatelliteCode: "SAT-2", SupportedBandsJSON: `["X"]`, MinimumContactSec: 600},
	}
	windows := []model.ContactWindow{
		{ID: 1, StationID: 1, SatelliteID: 1, StartAt: base, EndAt: base.Add(8 * time.Minute), Band: "S", Version: 1},
		{ID: 2, StationID: 1, SatelliteID: 1, StartAt: base.Add(6 * time.Minute), EndAt: base.Add(12 * time.Minute), Band: "S", Version: 1},
		{ID: 3, StationID: 1, SatelliteID: 2, StartAt: base.Add(20 * time.Minute), EndAt: base.Add(24 * time.Minute), Band: "X", Version: 1},
	}
	context := DetectionContext{Windows: windows, Stations: map[uint]model.GroundStation{1: station}, Satellites: assets}
	first, second := Detect(context), Detect(context)
	if len(first) < 4 {
		t.Fatalf("expected at least four conflict groups, got %d", len(first))
	}
	if len(first) != len(second) {
		t.Fatal("same input must produce same group count")
	}
	types := map[string]bool{}
	for index := range first {
		types[first[index].ConflictType] = true
		if first[index].Key != second[index].Key {
			t.Fatalf("key order changed at %d", index)
		}
	}
	for _, required := range []string{constants.ConflictTypeStationCapacity, constants.ConflictTypeSatelliteOverlap, constants.ConflictTypeBandMismatch, constants.ConflictTypeDurationShortfall} {
		if !types[required] {
			t.Fatalf("missing %s conflict", required)
		}
	}
}

func TestStableSuggestionRankingAndLockedProtection(t *testing.T) {
	base := time.Now().UTC()
	station := model.GroundStation{ID: 1, StationCode: "A", Latitude: 0, Longitude: 0, AntennaCount: 1, SupportedBandsJSON: `["S"]`, StationStatus: "active"}
	backup := model.GroundStation{ID: 2, StationCode: "B", Latitude: 1, Longitude: 1, AntennaCount: 1, SupportedBandsJSON: `["S"]`, StationStatus: "active"}
	asset := model.SatelliteAsset{ID: 1, SatelliteCode: "SAT", PriorityWeight: 5, SupportedBandsJSON: `["S"]`}
	group := newGroup(constants.ConflictTypeStationCapacity, []model.ContactWindow{
		{ID: 2, StationID: 1, SatelliteID: 1, StartAt: base, EndAt: base.Add(time.Minute), Band: "S", Priority: 3, Version: 1},
		{ID: 1, StationID: 1, SatelliteID: 1, StartAt: base, EndAt: base.Add(time.Minute), Band: "S", Priority: 2, Locked: true, Version: 1},
	}, 1, 2, 0, "capacity", nil)
	generator := NewCandidateGenerator(Weights{PriorityLoss: 4, MovementDistance: .02, ContactDuration: .003, ResourceMargin: 2}, []model.GroundStation{station, backup}, []model.SatelliteAsset{asset}, group.Windows)
	first, second := generator.Generate(group), generator.Generate(group)
	if len(first) != len(second) {
		t.Fatal("suggestion count changed")
	}
	for index := range first {
		if first[index].ActionKey != second[index].ActionKey {
			t.Fatal("suggestion ordering is not stable")
		}
	}
	for _, suggestion := range first {
		for _, moved := range suggestion.MoveWindowIDs {
			if moved == 1 {
				t.Fatal("locked window was suggested for movement")
			}
		}
	}
}
