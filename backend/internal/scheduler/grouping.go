package scheduler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"satellite-contact-window-deconfliction/backend/internal/constants"
	"satellite-contact-window-deconfliction/backend/internal/model"
)

type ConflictGroup struct {
	Key             string
	ConflictType    string
	Windows         []model.ContactWindow
	Capacity        int
	PeakConcurrency int
	BufferSeconds   int
	Summary         string
	Metadata        map[string]any
}

type DetectionContext struct {
	Windows    []model.ContactWindow
	Stations   map[uint]model.GroundStation
	Satellites map[uint]model.SatelliteAsset
}

func Detect(context DetectionContext) []ConflictGroup {
	groups := make([]ConflictGroup, 0)
	byStation := map[uint][]model.ContactWindow{}
	bySatellite := map[uint][]model.ContactWindow{}
	for _, window := range context.Windows {
		byStation[window.StationID] = append(byStation[window.StationID], window)
		bySatellite[window.SatelliteID] = append(bySatellite[window.SatelliteID], window)
	}
	stationIDs := sortedKeys(byStation)
	for _, stationID := range stationIDs {
		station, ok := context.Stations[stationID]
		if !ok {
			continue
		}
		windows := byStation[stationID]
		raw := SweepCapacity(windows, station.AntennaCount, 0)
		rawSets := map[string]bool{}
		for _, conflictWindows := range raw {
			rawSets[windowSetKey(conflictWindows)] = true
			groups = append(groups, newGroup(constants.ConflictTypeStationCapacity, conflictWindows, station.AntennaCount, len(conflictWindows), 0,
				fmt.Sprintf("%s exceeds %d available antenna channel(s)", station.StationCode, station.AntennaCount),
				map[string]any{"station_id": station.ID, "station_code": station.StationCode}))
		}
		expanded := SweepCapacity(windows, station.AntennaCount, time.Duration(station.SlewBufferSec)*time.Second)
		for _, conflictWindows := range expanded {
			if rawSets[windowSetKey(conflictWindows)] {
				continue
			}
			groups = append(groups, newGroup(constants.ConflictTypeSlewBuffer, conflictWindows, station.AntennaCount, len(conflictWindows), station.SlewBufferSec,
				fmt.Sprintf("%s requires a %d second slew buffer", station.StationCode, station.SlewBufferSec),
				map[string]any{"station_id": station.ID, "station_code": station.StationCode}))
		}
	}
	for _, satelliteID := range sortedKeys(bySatellite) {
		windows := bySatellite[satelliteID]
		for _, conflictWindows := range SweepCapacity(windows, 1, 0) {
			asset := context.Satellites[satelliteID]
			groups = append(groups, newGroup(constants.ConflictTypeSatelliteOverlap, conflictWindows, 1, len(conflictWindows), 0,
				fmt.Sprintf("%s has overlapping station opportunities", asset.SatelliteCode),
				map[string]any{"satellite_id": asset.ID, "satellite_code": asset.SatelliteCode}))
		}
	}
	for _, window := range context.Windows {
		station, stationOK := context.Stations[window.StationID]
		asset, assetOK := context.Satellites[window.SatelliteID]
		if !stationOK || !assetOK {
			continue
		}
		if !containsBand(station.SupportedBandsJSON, window.Band) || !containsBand(asset.SupportedBandsJSON, window.Band) {
			groups = append(groups, newGroup(constants.ConflictTypeBandMismatch, []model.ContactWindow{window}, 0, 0, 0,
				fmt.Sprintf("%s band is not supported by both %s and %s", window.Band, station.StationCode, asset.SatelliteCode),
				map[string]any{"station_bands": station.SupportedBandsJSON, "satellite_bands": asset.SupportedBandsJSON, "requested_band": window.Band}))
		}
		if window.DurationSec() < asset.MinimumContactSec {
			groups = append(groups, newGroup(constants.ConflictTypeDurationShortfall, []model.ContactWindow{window}, asset.MinimumContactSec, window.DurationSec(), 0,
				fmt.Sprintf("Window duration is %d seconds below %s minimum", asset.MinimumContactSec-window.DurationSec(), asset.SatelliteCode),
				map[string]any{"minimum_contact_sec": asset.MinimumContactSec, "actual_contact_sec": window.DurationSec()}))
		}
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].ConflictType != groups[j].ConflictType {
			return conflictRank(groups[i].ConflictType) < conflictRank(groups[j].ConflictType)
		}
		return groups[i].Key < groups[j].Key
	})
	return groups
}

func newGroup(conflictType string, windows []model.ContactWindow, capacity, peak, buffer int, summary string, metadata map[string]any) ConflictGroup {
	sort.Slice(windows, func(i, j int) bool { return windows[i].ID < windows[j].ID })
	parts := []string{conflictType}
	for _, window := range windows {
		parts = append(parts, fmt.Sprintf("%d@%d", window.ID, window.Version))
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return ConflictGroup{
		Key: hex.EncodeToString(digest[:12]), ConflictType: conflictType, Windows: windows,
		Capacity: capacity, PeakConcurrency: peak, BufferSeconds: buffer, Summary: summary, Metadata: metadata,
	}
}

func containsBand(encoded, band string) bool {
	clean := strings.NewReplacer("[", "", "]", "", "\"", "", " ", "").Replace(encoded)
	for _, value := range strings.Split(clean, ",") {
		if value == band {
			return true
		}
	}
	return false
}

func sortedKeys(values map[uint][]model.ContactWindow) []uint {
	keys := make([]uint, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func conflictRank(value string) int {
	for index, candidate := range constants.ConflictTypes {
		if value == candidate {
			return index
		}
	}
	return len(constants.ConflictTypes)
}
