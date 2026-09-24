export type WindowStatus = 'candidate' | 'submitted' | 'locked' | 'allocated' | 'cancelled';

export const WINDOW_STATUSES: WindowStatus[] = ['candidate', 'submitted', 'locked', 'allocated', 'cancelled'];

export interface ContactWindow {
  id: number;
  station_id: number;
  station_code: string;
  satellite_id: number;
  satellite_code: string;
  start_at: string;
  end_at: string;
  duration_sec: number;
  band: string;
  elevation_peak_deg: number;
  window_status: WindowStatus;
  priority: number;
  locked: boolean;
  source_version: string;
  version: number;
  updated_at: string;
}

export interface ContactWindowInput {
  station_id: number;
  satellite_id: number;
  start_at: string;
  end_at: string;
  band: string;
  elevation_peak_deg: number;
  priority: number;
  source_version: string;
}
