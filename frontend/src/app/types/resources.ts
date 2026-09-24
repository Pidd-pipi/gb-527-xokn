export interface GroundStation {
  id: number;
  station_code: string;
  name: string;
  latitude: number;
  longitude: number;
  antenna_count: number;
  supported_bands: string[];
  slew_buffer_sec: number;
  station_status: string;
  version: number;
  updated_at: string;
}

export interface SatelliteAsset {
  id: number;
  satellite_code: string;
  name: string;
  orbit_class: string;
  supported_bands: string[];
  priority_weight: number;
  minimum_contact_sec: number;
  asset_status: string;
  version: number;
  updated_at: string;
}

export interface AuditEvent {
  id: number;
  actor: string;
  role: string;
  action: string;
  resource_type: string;
  resource_id: string;
  request_id: string;
  parameters: Record<string, unknown>;
  before_summary: Record<string, unknown>;
  after_summary: Record<string, unknown>;
  created_at: string;
}
