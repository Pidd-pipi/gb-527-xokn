import { ChangeDetectionStrategy, Component, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSnackBar, MatSnackBarModule } from '@angular/material/snack-bar';
import { forkJoin, finalize } from 'rxjs';
import { ApiService } from '../api/api.service';
import { GroundStation } from '../types/resources';
import { ContactWindow } from '../types/window';
import { ResourceTimelineComponent } from '../components/common/resource-timeline.component';
import { useAuth } from '../hooks/use-auth';
import { apiErrorMessage } from '../hooks/use-conflict-detection';

@Component({
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, MatSelectModule, MatSnackBarModule, ResourceTimelineComponent],
  template: `
    <div class="page">
      <header class="page-head"><div><span class="eyebrow">Resource capacity</span><h1>Ground stations</h1><p>{{ stations().length }} stations · {{ totalAntennas() }} antenna channels</p></div>
        <div class="action-row"><button *ngIf="auth.canPlan()" mat-flat-button color="primary" (click)="newStation()">{{ formOpen() ? 'Close editor' : 'New station' }}</button></div></header>
      <div class="error-banner" *ngIf="error()"><span>{{ error() }}</span><button mat-button (click)="load()">Retry</button></div>
      <form class="inline-form" *ngIf="formOpen()" [formGroup]="form" (ngSubmit)="save()">
        <mat-form-field class="span-2" appearance="outline"><mat-label>Station code</mat-label><input matInput formControlName="station_code"></mat-form-field>
        <mat-form-field class="span-4" appearance="outline"><mat-label>Name</mat-label><input matInput formControlName="name"></mat-form-field>
        <mat-form-field class="span-3" appearance="outline"><mat-label>Latitude</mat-label><input matInput type="number" formControlName="latitude"></mat-form-field>
        <mat-form-field class="span-3" appearance="outline"><mat-label>Longitude</mat-label><input matInput type="number" formControlName="longitude"></mat-form-field>
        <mat-form-field class="span-2" appearance="outline"><mat-label>Antennas</mat-label><input matInput type="number" formControlName="antenna_count"></mat-form-field>
        <mat-form-field class="span-3" appearance="outline"><mat-label>Bands</mat-label><mat-select multiple formControlName="supported_bands"><mat-option *ngFor="let band of bands" [value]="band">{{ band }}</mat-option></mat-select></mat-form-field>
        <mat-form-field class="span-3" appearance="outline"><mat-label>Slew buffer (sec)</mat-label><input matInput type="number" formControlName="slew_buffer_sec"></mat-form-field>
        <mat-form-field class="span-2" appearance="outline"><mat-label>Status</mat-label><mat-select formControlName="station_status"><mat-option value="active">Active</mat-option><mat-option value="maintenance">Maintenance</mat-option><mat-option value="retired">Retired</mat-option></mat-select></mat-form-field>
        <div class="span-2 form-actions"><button mat-button type="button" (click)="cancel()">Cancel</button><button mat-flat-button color="primary" type="submit" [disabled]="form.invalid || saving()">{{ editing() ? 'Update' : 'Create' }}</button></div>
      </form>
      <div class="metric-strip"><div class="metric"><strong>{{ totalAntennas() }}</strong><span>antenna channels</span></div><div class="metric"><strong>{{ activeCount() }}</strong><span>active stations</span></div><div class="metric"><strong>{{ lockedCount() }}</strong><span>locked windows</span></div></div>
      <div class="section-title"><h2>Capacity register</h2><span>Select a row to edit</span></div>
      <div class="surface"><table class="data-table"><thead><tr><th>Code</th><th>Station</th><th>Coordinates</th><th>Antennas</th><th>Bands</th><th>Slew</th><th>Status</th></tr></thead>
        <tbody><tr *ngFor="let station of stations()" [class.selected]="editing()?.id === station.id" (click)="edit(station)"><td class="code">{{ station.station_code }}</td><td><strong>{{ station.name }}</strong></td><td>{{ station.latitude | number:'1.2-2' }}, {{ station.longitude | number:'1.2-2' }}</td><td>{{ station.antenna_count }}</td><td><span class="pill" *ngFor="let band of station.supported_bands">{{ band }}</span></td><td>{{ station.slew_buffer_sec }} sec</td><td>{{ station.station_status }}</td></tr></tbody>
      </table><div class="empty" *ngIf="!stations().length && !loading()">No stations in the planning register.</div></div>
      <div class="section-title"><h2>Station occupancy</h2><span>UTC · slew buffer shown in conflict analysis</span></div>
      <div class="surface"><app-resource-timeline [windows]="windows()" groupBy="station" /></div>
    </div>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class StationsPage implements OnInit {
  readonly auth = useAuth();
  readonly stations = signal<GroundStation[]>([]); readonly windows = signal<ContactWindow[]>([]);
  readonly loading = signal(false); readonly saving = signal(false); readonly error = signal(''); readonly formOpen = signal(false); readonly editing = signal<GroundStation | null>(null);
  readonly bands = ['S', 'X', 'Ka', 'Ku'];
  private readonly builder = new FormBuilder().nonNullable;
  readonly form = this.builder.group({
    station_code: ['', [Validators.required, Validators.minLength(3)]], name: ['', [Validators.required, Validators.minLength(2)]], latitude: [0, [Validators.min(-90), Validators.max(90)]],
    longitude: [0, [Validators.min(-180), Validators.max(180)]], antenna_count: [1, [Validators.required, Validators.min(1), Validators.max(32)]], supported_bands: [['S'] as string[], Validators.required],
    slew_buffer_sec: [120, [Validators.min(0), Validators.max(3600)]], station_status: ['active', Validators.required],
  });
  constructor(private readonly api: ApiService, private readonly snackBar: MatSnackBar) {}
  ngOnInit(): void { this.load(); }
  load(): void { this.loading.set(true); this.error.set(''); forkJoin([this.api.stations({ page_size: 100 }), this.api.windows({ page_size: 200 })]).pipe(finalize(() => this.loading.set(false))).subscribe({ next: ([stations, windows]) => { this.stations.set(stations.data); this.windows.set(windows.data); }, error: (error: unknown) => this.error.set(apiErrorMessage(error)) }); }
  totalAntennas(): number { return this.stations().reduce((sum, station) => sum + station.antenna_count, 0); }
  activeCount(): number { return this.stations().filter((station) => station.station_status === 'active').length; }
  lockedCount(): number { return this.windows().filter((window) => window.locked).length; }
  newStation(): void { if (this.formOpen() && !this.editing()) { this.cancel(); return; } this.editing.set(null); this.form.reset({ station_code: '', name: '', latitude: 0, longitude: 0, antenna_count: 1, supported_bands: ['S'], slew_buffer_sec: 120, station_status: 'active' }); this.form.controls.station_code.enable(); this.formOpen.set(true); }
  edit(station: GroundStation): void { if (!this.auth.canPlan()) return; this.editing.set(station); this.form.setValue({ station_code: station.station_code, name: station.name, latitude: station.latitude, longitude: station.longitude, antenna_count: station.antenna_count, supported_bands: station.supported_bands, slew_buffer_sec: station.slew_buffer_sec, station_status: station.station_status }); this.form.controls.station_code.disable(); this.formOpen.set(true); }
  cancel(): void { this.editing.set(null); this.formOpen.set(false); this.form.controls.station_code.enable(); }
  save(): void {
    if (this.form.invalid) return; this.saving.set(true); const value = this.form.getRawValue(); const current = this.editing();
    const request = current ? this.api.updateStation(current.id, { name: value.name, latitude: value.latitude, longitude: value.longitude, antenna_count: value.antenna_count, supported_bands: value.supported_bands, slew_buffer_sec: value.slew_buffer_sec, station_status: value.station_status, expected_version: current.version }) : this.api.createStation(value as Omit<GroundStation, 'id' | 'version' | 'updated_at'>);
    request.pipe(finalize(() => this.saving.set(false))).subscribe({ next: () => { this.snackBar.open(current ? 'Station updated' : 'Station created', 'Close', { duration: 2200 }); this.cancel(); this.load(); }, error: (error: unknown) => this.error.set(apiErrorMessage(error)) });
  }
}
