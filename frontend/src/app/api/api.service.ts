import { Injectable, inject } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { ApiEnvelope, LoginResponse, PageEnvelope } from '../types/api';
import { AuditEvent, GroundStation, SatelliteAsset } from '../types/resources';
import { ContactWindow, ContactWindowInput } from '../types/window';
import { ConflictResolution, DetectionResult } from '../types/conflict';

@Injectable({ providedIn: 'root' })
export class ApiService {
  private readonly http = inject(HttpClient);
  private readonly root = '/api/v1';

  login(username: string, password: string): Observable<ApiEnvelope<LoginResponse>> {
    return this.http.post<ApiEnvelope<LoginResponse>>(`${this.root}/auth/login`, { username, password });
  }

  stations(params: Record<string, string | number> = {}): Observable<PageEnvelope<GroundStation>> {
    return this.http.get<PageEnvelope<GroundStation>>(`${this.root}/stations`, { params: this.params(params) });
  }

  createStation(input: Omit<GroundStation, 'id' | 'version' | 'updated_at'>): Observable<ApiEnvelope<GroundStation>> {
    return this.http.post<ApiEnvelope<GroundStation>>(`${this.root}/stations`, input);
  }

  updateStation(id: number, input: Record<string, unknown>): Observable<ApiEnvelope<GroundStation>> {
    return this.http.put<ApiEnvelope<GroundStation>>(`${this.root}/stations/${id}`, input);
  }

  satellites(params: Record<string, string | number> = {}): Observable<PageEnvelope<SatelliteAsset>> {
    return this.http.get<PageEnvelope<SatelliteAsset>>(`${this.root}/satellites`, { params: this.params(params) });
  }

  createSatellite(input: Omit<SatelliteAsset, 'id' | 'version' | 'updated_at'>): Observable<ApiEnvelope<SatelliteAsset>> {
    return this.http.post<ApiEnvelope<SatelliteAsset>>(`${this.root}/satellites`, input);
  }

  updateSatellite(id: number, input: Record<string, unknown>): Observable<ApiEnvelope<SatelliteAsset>> {
    return this.http.put<ApiEnvelope<SatelliteAsset>>(`${this.root}/satellites/${id}`, input);
  }

  windows(params: Record<string, string | number> = {}): Observable<PageEnvelope<ContactWindow>> {
    return this.http.get<PageEnvelope<ContactWindow>>(`${this.root}/windows`, { params: this.params(params) });
  }

  createWindow(input: ContactWindowInput): Observable<ApiEnvelope<ContactWindow>> {
    return this.http.post<ApiEnvelope<ContactWindow>>(`${this.root}/windows`, input);
  }

  submitWindow(id: number, expectedVersion: number): Observable<ApiEnvelope<ContactWindow>> {
    return this.http.post<ApiEnvelope<ContactWindow>>(`${this.root}/windows/${id}/submit`, { expected_version: expectedVersion });
  }

  lockWindow(id: number, expectedVersion: number): Observable<ApiEnvelope<ContactWindow>> {
    return this.http.post<ApiEnvelope<ContactWindow>>(`${this.root}/windows/${id}/lock`, { expected_version: expectedVersion });
  }

  conflicts(params: Record<string, string | number> = {}): Observable<PageEnvelope<ConflictResolution>> {
    return this.http.get<PageEnvelope<ConflictResolution>>(`${this.root}/conflicts`, { params: this.params(params) });
  }

  conflict(id: number): Observable<ApiEnvelope<ConflictResolution>> {
    return this.http.get<ApiEnvelope<ConflictResolution>>(`${this.root}/conflicts/${id}`);
  }

  detectConflicts(from: string, to: string): Observable<ApiEnvelope<DetectionResult>> {
    return this.http.post<ApiEnvelope<DetectionResult>>(`${this.root}/conflicts/detect`, { from, to });
  }

  submitConflict(id: number, expectedVersion: number): Observable<ApiEnvelope<ConflictResolution>> {
    return this.http.post<ApiEnvelope<ConflictResolution>>(`${this.root}/conflicts/${id}/submit`, { expected_version: expectedVersion });
  }

  reviewConflict(id: number, expectedVersion: number, decision: 'accepted' | 'rejected', actionKey: string, reviewNote: string): Observable<ApiEnvelope<ConflictResolution>> {
    return this.http.post<ApiEnvelope<ConflictResolution>>(`${this.root}/conflicts/${id}/review`, {
      expected_version: expectedVersion,
      decision,
      action_key: actionKey,
      review_note: reviewNote,
    });
  }

  audit(params: Record<string, string | number> = {}): Observable<PageEnvelope<AuditEvent>> {
    return this.http.get<PageEnvelope<AuditEvent>>(`${this.root}/audit`, { params: this.params(params) });
  }

  private params(values: Record<string, string | number>): HttpParams {
    let result = new HttpParams();
    Object.entries(values).forEach(([key, value]) => {
      if (value !== '' && value !== 0) result = result.set(key, String(value));
    });
    return result;
  }
}
