import { ChangeDetectionStrategy, Component, OnInit, computed, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSnackBar, MatSnackBarModule } from '@angular/material/snack-bar';
import { finalize, forkJoin } from 'rxjs';
import { ApiService } from '../api/api.service';
import { ContactWindow, ContactWindowInput, WINDOW_STATUSES } from '../types/window';
import { GroundStation, SatelliteAsset } from '../types/resources';
import { ResourceTimelineComponent } from '../components/common/resource-timeline.component';
import { WindowStatusBadgeComponent } from '../components/common/window-status-badge.component';
import { useAuth } from '../hooks/use-auth';
import { apiErrorMessage } from '../hooks/use-conflict-detection';
import { formatUtc, toLocalInput } from '../utils/date';

@Component({
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, MatSelectModule, MatSnackBarModule, ResourceTimelineComponent, WindowStatusBadgeComponent],
  template: `
    <div class="page">
      <header class="page-head"><div><span class="eyebrow">UTC opportunity board</span><h1>Contact windows</h1><p>{{ filtered().length }} of {{ windows().length }} visible</p></div><button *ngIf="auth.canPlan()" mat-flat-button color="primary" (click)="toggleForm()">{{ formOpen() ? 'Close editor' : 'New window' }}</button></header>
      <div class="error-banner" *ngIf="error()"><span>{{ error() }}</span><button mat-button (click)="load()">Reload</button></div>
      <form class="inline-form" *ngIf="formOpen()" [formGroup]="form" (ngSubmit)="create()">
        <mat-form-field class="span-3" appearance="outline"><mat-label>Ground station</mat-label><mat-select formControlName="station_id"><mat-option *ngFor="let station of stations()" [value]="station.id">{{ station.station_code }} · {{ station.name }}</mat-option></mat-select></mat-form-field>
        <mat-form-field class="span-3" appearance="outline"><mat-label>Satellite</mat-label><mat-select formControlName="satellite_id"><mat-option *ngFor="let asset of satellites()" [value]="asset.id">{{ asset.satellite_code }} · {{ asset.name }}</mat-option></mat-select></mat-form-field>
        <mat-form-field class="span-3" appearance="outline"><mat-label>Start</mat-label><input matInput type="datetime-local" formControlName="start_at"></mat-form-field>
        <mat-form-field class="span-3" appearance="outline"><mat-label>End</mat-label><input matInput type="datetime-local" formControlName="end_at"></mat-form-field>
        <mat-form-field class="span-2" appearance="outline"><mat-label>Band</mat-label><mat-select formControlName="band"><mat-option *ngFor="let band of bands" [value]="band">{{ band }}</mat-option></mat-select></mat-form-field>
        <mat-form-field class="span-2" appearance="outline"><mat-label>Peak elevation</mat-label><input matInput type="number" formControlName="elevation_peak_deg"></mat-form-field>
        <mat-form-field class="span-2" appearance="outline"><mat-label>Priority</mat-label><input matInput type="number" formControlName="priority"></mat-form-field>
        <mat-form-field class="span-4" appearance="outline"><mat-label>Source version</mat-label><input matInput formControlName="source_version"></mat-form-field>
        <div class="span-2 form-actions"><button mat-button type="button" (click)="toggleForm()">Cancel</button><button mat-flat-button color="primary" type="submit" [disabled]="form.invalid || saving()">Create</button></div>
      </form>
      <div class="inline-form filter-bar">
        <mat-form-field class="span-3" appearance="outline"><mat-label>Station</mat-label><mat-select [value]="stationFilter()" (selectionChange)="stationFilter.set($event.value)"><mat-option [value]="0">All stations</mat-option><mat-option *ngFor="let station of stations()" [value]="station.id">{{ station.station_code }}</mat-option></mat-select></mat-form-field>
        <mat-form-field class="span-3" appearance="outline"><mat-label>Satellite</mat-label><mat-select [value]="satelliteFilter()" (selectionChange)="satelliteFilter.set($event.value)"><mat-option [value]="0">All satellites</mat-option><mat-option *ngFor="let asset of satellites()" [value]="asset.id">{{ asset.satellite_code }}</mat-option></mat-select></mat-form-field>
        <mat-form-field class="span-3" appearance="outline"><mat-label>Status</mat-label><mat-select [value]="statusFilter()" (selectionChange)="statusFilter.set($event.value)"><mat-option value="">All statuses</mat-option><mat-option *ngFor="let status of statuses" [value]="status">{{ status }}</mat-option></mat-select></mat-form-field>
        <div class="span-3 filter-summary"><strong>{{ lockedCount() }}</strong><span>locked planning inputs</span></div>
      </div>
      <div class="section-title"><h2>Resource timeline</h2><span>Candidate placement · UTC</span></div><div class="surface"><app-resource-timeline [windows]="filtered()" groupBy="station" /></div>
      <div class="section-title"><h2>Window register</h2><span>Algorithm suggestions never write to this register</span></div>
      <div class="surface"><table class="data-table"><thead><tr><th>ID</th><th>Station / satellite</th><th>Interval</th><th>Band</th><th>Priority</th><th>Status</th><th>Version</th><th *ngIf="auth.canPlan()">Actions</th></tr></thead>
        <tbody><tr *ngFor="let window of filtered()"><td class="code">#{{ window.id }}</td><td><strong>{{ window.station_code }}</strong><br><span class="muted">{{ window.satellite_code }}</span></td><td>{{ format(window.start_at) }}<br><span class="muted">{{ window.duration_sec }} sec</span></td><td>{{ window.band }}</td><td>P{{ window.priority }}</td><td><app-window-status-badge [status]="window.window_status" /></td><td>v{{ window.version }}</td><td *ngIf="auth.canPlan()"><div class="action-row"><button mat-button *ngIf="window.window_status === 'candidate'" (click)="submitWindow(window)">Submit</button><button mat-stroked-button *ngIf="!window.locked && (window.window_status === 'candidate' || window.window_status === 'submitted')" (click)="lockWindow(window)">Lock</button></div></td></tr></tbody>
      </table><div class="empty" *ngIf="!filtered().length">No windows match these filters.</div></div>
    </div>
  `,
  styles: [`.filter-bar { padding-bottom: 4px; background: transparent; border: 0; border-bottom: 1px solid #ccd3ce; } .filter-summary { min-height: 56px; display: flex; flex-direction: column; justify-content: center; } .filter-summary strong { font-size: 20px; } .filter-summary span { color: #66716d; font-size: 10px; text-transform: uppercase; }`],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WindowsPage implements OnInit {
  readonly auth = useAuth(); readonly stations = signal<GroundStation[]>([]); readonly satellites = signal<SatelliteAsset[]>([]); readonly windows = signal<ContactWindow[]>([]);
  readonly stationFilter = signal(0); readonly satelliteFilter = signal(0); readonly statusFilter = signal(''); readonly formOpen = signal(false); readonly saving = signal(false); readonly error = signal('');
  readonly statuses = WINDOW_STATUSES; readonly bands = ['S', 'X', 'Ka', 'Ku']; private readonly builder = new FormBuilder().nonNullable;
  readonly filtered = computed(() => this.windows().filter((window) => (!this.stationFilter() || window.station_id === this.stationFilter()) && (!this.satelliteFilter() || window.satellite_id === this.satelliteFilter()) && (!this.statusFilter() || window.window_status === this.statusFilter())));
  readonly form = this.builder.group({ station_id: [0, [Validators.required, Validators.min(1)]], satellite_id: [0, [Validators.required, Validators.min(1)]], start_at: ['', Validators.required], end_at: ['', Validators.required], band: ['S', Validators.required], elevation_peak_deg: [45, [Validators.min(0), Validators.max(90)]], priority: [5, [Validators.min(0), Validators.max(10)]], source_version: ['orbit-2026.234-ui', [Validators.required, Validators.minLength(3)]] });
  constructor(private readonly api: ApiService, private readonly snackBar: MatSnackBar) {}
  ngOnInit(): void { this.resetDates(); this.load(); }
  load(): void { this.error.set(''); forkJoin([this.api.stations({ page_size: 100 }), this.api.satellites({ page_size: 100 }), this.api.windows({ page_size: 200 })]).subscribe({ next: ([stations, satellites, windows]) => { this.stations.set(stations.data); this.satellites.set(satellites.data); this.windows.set(windows.data); }, error: (error: unknown) => this.error.set(apiErrorMessage(error)) }); }
  toggleForm(): void { this.formOpen.update((value) => !value); }
  lockedCount(): number { return this.filtered().filter((window) => window.locked).length; }
  format(value: string): string { return formatUtc(value); }
  create(): void {
    if (this.form.invalid) return; this.saving.set(true); const value = this.form.getRawValue();
    const input: ContactWindowInput = { ...value, start_at: new Date(value.start_at).toISOString(), end_at: new Date(value.end_at).toISOString() };
    this.api.createWindow(input).pipe(finalize(() => this.saving.set(false))).subscribe({ next: () => { this.snackBar.open('Candidate window created', 'Close', { duration: 2200 }); this.formOpen.set(false); this.resetDates(); this.load(); }, error: (error: unknown) => this.error.set(apiErrorMessage(error)) });
  }
  submitWindow(window: ContactWindow): void { this.api.submitWindow(window.id, window.version).subscribe({ next: () => { this.snackBar.open('Window submitted', 'Close', { duration: 2000 }); this.load(); }, error: (error: unknown) => this.error.set(apiErrorMessage(error)) }); }
  lockWindow(window: ContactWindow): void { this.api.lockWindow(window.id, window.version).subscribe({ next: () => { this.snackBar.open('Window locked', 'Close', { duration: 2000 }); this.load(); }, error: (error: unknown) => this.error.set(apiErrorMessage(error)) }); }
  private resetDates(): void { const start = new Date(Date.now() + 2 * 60 * 60 * 1000); const end = new Date(start.getTime() + 10 * 60 * 1000); this.form.patchValue({ start_at: toLocalInput(start), end_at: toLocalInput(end) }); }
}
