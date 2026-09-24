package scheduler

import (
	"math"
	"sort"
)

type Weights struct {
	PriorityLoss     float64 `json:"priority_loss"`
	MovementDistance float64 `json:"movement_distance"`
	ContactDuration  float64 `json:"contact_duration"`
	ResourceMargin   float64 `json:"resource_margin"`
}

type ScoreBreakdown struct {
	PriorityLoss       float64 `json:"priority_loss"`
	MovementDistanceKM float64 `json:"movement_distance_km"`
	ContactDurationSec int     `json:"contact_duration_sec"`
	ResourceMargin     int     `json:"resource_margin"`
	TotalScore         float64 `json:"total_score"`
}

type Suggestion struct {
	ActionKey         string         `json:"action_key"`
	ActionType        string         `json:"action_type"`
	Title             string         `json:"title"`
	Rationale         string         `json:"rationale"`
	KeepWindowIDs     []uint         `json:"keep_window_ids"`
	MoveWindowIDs     []uint         `json:"move_window_ids"`
	TargetStationID   *uint          `json:"target_station_id,omitempty"`
	AlternateWindowID *uint          `json:"alternate_window_id,omitempty"`
	RequiresManual    bool           `json:"requires_manual"`
	Score             ScoreBreakdown `json:"score"`
}

func Score(weights Weights, priorityLoss, distance float64, duration, margin int) ScoreBreakdown {
	total := float64(duration)*weights.ContactDuration + float64(margin)*weights.ResourceMargin - priorityLoss*weights.PriorityLoss - distance*weights.MovementDistance
	return ScoreBreakdown{
		PriorityLoss: round(priorityLoss, 3), MovementDistanceKM: round(distance, 2), ContactDurationSec: duration,
		ResourceMargin: margin, TotalScore: round(total, 4),
	}
}

func StableSortSuggestions(suggestions []Suggestion) {
	sort.SliceStable(suggestions, func(i, j int) bool {
		if suggestions[i].Score.TotalScore != suggestions[j].Score.TotalScore {
			return suggestions[i].Score.TotalScore > suggestions[j].Score.TotalScore
		}
		return suggestions[i].ActionKey < suggestions[j].ActionKey
	})
}

func round(value float64, digits int) float64 {
	factor := math.Pow10(digits)
	return math.Round(value*factor) / factor
}
