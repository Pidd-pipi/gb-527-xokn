export type ConflictType = 'station_capacity' | 'satellite_overlap' | 'band_mismatch' | 'duration_shortfall' | 'slew_buffer';
export type ResolutionStatus = 'detected' | 'proposed' | 'pending_review' | 'accepted' | 'rejected';

export const CONFLICT_TYPES: ConflictType[] = ['station_capacity', 'satellite_overlap', 'band_mismatch', 'duration_shortfall', 'slew_buffer'];

export interface ScoreBreakdown {
  priority_loss: number;
  movement_distance_km: number;
  contact_duration_sec: number;
  resource_margin: number;
  total_score: number;
}

export interface ResolutionSuggestion {
  action_key: string;
  action_type: string;
  title: string;
  rationale: string;
  keep_window_ids: number[];
  move_window_ids: number[];
  target_station_id?: number;
  alternate_window_id?: number;
  requires_manual: boolean;
  score: ScoreBreakdown;
}

export interface ConflictEvidence {
  summary: string;
  window_facts: Array<Record<string, unknown>>;
  capacity: number;
  peak_concurrency: number;
  buffer_seconds: number;
  metadata: Record<string, unknown>;
}

export interface ConflictResolution {
  id: number;
  conflict_key: string;
  window_ids: number[];
  conflict_type: ConflictType;
  evidence: ConflictEvidence;
  suggestions: ResolutionSuggestion[];
  selected_action?: ResolutionSuggestion;
  resolution_status: ResolutionStatus;
  resolved_by: string;
  review_note: string;
  version: number;
  resolved_at?: string;
  created_at: string;
  updated_at: string;
}

export interface DetectionResult {
  range_from: string;
  range_to: string;
  window_count: number;
  conflict_count: number;
  resolutions: ConflictResolution[];
}
