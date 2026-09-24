import { Injectable, signal } from '@angular/core';
import { ContactWindow } from '../types/window';
import { GroundStation, SatelliteAsset } from '../types/resources';

@Injectable({ providedIn: 'root' })
export class PlanningStore {
  readonly stations = signal<GroundStation[]>([]);
  readonly satellites = signal<SatelliteAsset[]>([]);
  readonly windows = signal<ContactWindow[]>([]);
  readonly loading = signal(false);
}
