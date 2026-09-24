import { ChangeDetectionStrategy, Component, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSnackBar, MatSnackBarModule } from '@angular/material/snack-bar';
import { finalize, forkJoin } from 'rxjs';
import { ApiService } from '../api/api.service';
import { GroundStation, SatelliteAsset } from '../types/resources';
import { ContactWindow } from '../types/window';
import { ResourceTimelineComponent } from '../components/common/resource-timeline.component';
import { useAuth } from '../hooks/use-auth';
import { apiErrorMessage } from '../hooks/use-conflict-detection';

@Component({
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, MatSelectModule, MatSnackBarModule, ResourceTimelineComponent],
  template: `
    <div class="page">
      <header class="page-head"><div><span class="eyebrow">Contact demand</span><h1>Satellite assets</h1><p>Priority and minimum contact constraints</p></div><button *ngIf="auth.canPlan()" mat-flat-button color="primary" (click)="newAsset()">{{ formOpen() ? 'Close editor' : 'New satellite' }}</button></header>
      <div class="error-banner" *ngIf="error()"><span>{{ error() }}</span><button mat-button (click)="load()">Retry</button></div>
      <form class="inline-form" *ngIf="formOpen()" [formGroup]="form" (ngSubmit)="save()">
        <mat-form-field class="span-2" appearance="outline"><mat-label>Satellite code</mat-label><input matInput formControlName="satellite_code"></mat-form-field>
        <mat-form-field class="span-4" appearance="outline"><mat-label>Name</mat-label><input matInput formControlName="name"></mat-form-field>
        <mat-form-field class="span-2" appearance="outline"><mat-label>Orbit</mat-label><mat-select formControlName="orbit_class"><mat-option *ngFor="let orbit of orbits" [value]="orbit">{{ orbit }}</mat-option></mat-select></mat-form-field>
        <mat-form-field class="span-2" appearance="outline"><mat-label>Priority weight</mat-label><input matInput type="number" step="0.1" formControlName="priority_weight"></mat-form-field>
        <mat-form-field class="span-2" appearance="outline"><mat-label>Minimum (sec)</mat-label><input matInput type="number" formControlName="minimum_contact_sec"></mat-form-field>
        <mat-form-field class="span-4" appearance="outline"><mat-label>Bands</mat-label><mat-select multiple formControlName="supported_bands"><mat-option *ngFor="let band of bands" [value]="band">{{ band }}</mat-option></mat-select></mat-form-field>
        <mat-form-field class="span-3" appearance="outline"><mat-label>Status</mat-label><mat-select formControlName="asset_status"><mat-option value="active">Active</mat-option><mat-option value="standby">Standby</mat-option><mat-option value="retired">Retired</mat-option></mat-select></mat-form-field>
        <div class="span-5 form-actions"><button mat-button type="button" (click)="cancel()">Cancel</button><button mat-flat-button color="primary" type="submit" [disabled]="form.invalid || saving()">{{ editing() ? 'Update' : 'Create' }}</button></div>
      </form>
      <div class="metric-strip"><div class="metric"><strong>{{ satellites().length }}</strong><span>planning assets</span></div><div class="metric"><strong>{{ highPriority() }}</strong><span>priority ≥ 7</span></div><div class="metric"><strong>{{ averageMinimum() }}</strong><span>avg minimum sec</span></div><div class="metric"><strong>{{ stations().length }}</strong><span>station options</span></div></div>
      <div class="section-title"><h2>Demand register</h2><span>Versioned planning inputs</span></div>
      <div class="surface"><table class="data-table"><thead><tr><th>Code</th><th>Asset</th><th>Orbit</th><th>Bands</th><th>Weight</th><th>Minimum</th><th>Status</th></tr></thead>
        <tbody><tr *ngFor="let asset of satellites()" [class.selected]="editing()?.id === asset.id" (click)="edit(asset)"><td class="code">{{ asset.satellite_code }}</td><td><strong>{{ asset.name }}</strong></td><td>{{ asset.orbit_class }}</td><td><span class="pill" *ngFor="let band of asset.supported_bands">{{ band }}</span></td><td>{{ asset.priority_weight | number:'1.1-1' }}</td><td>{{ asset.minimum_contact_sec }} sec</td><td>{{ asset.asset_status }}</td></tr></tbody>
      </table><div class="empty" *ngIf="!satellites().length">No satellite assets in the register.</div></div>
      <div class="section-title"><h2>Demand timeline</h2><span>Grouped by planning asset</span></div><div class="surface"><app-resource-timeline [windows]="windows()" groupBy="satellite" /></div>
    </div>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SatellitesPage implements OnInit {
  readonly auth = useAuth(); readonly satellites = signal<SatelliteAsset[]>([]); readonly stations = signal<GroundStation[]>([]); readonly windows = signal<ContactWindow[]>([]);
  readonly error = signal(''); readonly saving = signal(false); readonly formOpen = signal(false); readonly editing = signal<SatelliteAsset | null>(null);
  readonly bands = ['S', 'X', 'Ka', 'Ku']; readonly orbits = ['LEO', 'MEO', 'GEO', 'HEO']; private readonly builder = new FormBuilder().nonNullable;
  readonly form = this.builder.group({ satellite_code: ['', [Validators.required, Validators.minLength(3)]], name: ['', [Validators.required, Validators.minLength(2)]], orbit_class: ['LEO', Validators.required], supported_bands: [['S'] as string[], Validators.required], priority_weight: [5, [Validators.min(0), Validators.max(100)]], minimum_contact_sec: [300, [Validators.min(30), Validators.max(86400)]], asset_status: ['active', Validators.required] });
  constructor(private readonly api: ApiService, private readonly snackBar: MatSnackBar) {}
  ngOnInit(): void { this.load(); }
  load(): void { this.error.set(''); forkJoin([this.api.satellites({ page_size: 100 }), this.api.stations({ page_size: 100 }), this.api.windows({ page_size: 200 })]).subscribe({ next: ([satellites, stations, windows]) => { this.satellites.set(satellites.data); this.stations.set(stations.data); this.windows.set(windows.data); }, error: (error: unknown) => this.error.set(apiErrorMessage(error)) }); }
  highPriority(): number { return this.satellites().filter((asset) => asset.priority_weight >= 7).length; }
  averageMinimum(): number { return this.satellites().length ? Math.round(this.satellites().reduce((sum, asset) => sum + asset.minimum_contact_sec, 0) / this.satellites().length) : 0; }
  newAsset(): void { if (this.formOpen() && !this.editing()) { this.cancel(); return; } this.editing.set(null); this.form.reset({ satellite_code: '', name: '', orbit_class: 'LEO', supported_bands: ['S'], priority_weight: 5, minimum_contact_sec: 300, asset_status: 'active' }); this.form.controls.satellite_code.enable(); this.formOpen.set(true); }
  edit(asset: SatelliteAsset): void { if (!this.auth.canPlan()) return; this.editing.set(asset); this.form.setValue({ satellite_code: asset.satellite_code, name: asset.name, orbit_class: asset.orbit_class, supported_bands: asset.supported_bands, priority_weight: asset.priority_weight, minimum_contact_sec: asset.minimum_contact_sec, asset_status: asset.asset_status }); this.form.controls.satellite_code.disable(); this.formOpen.set(true); }
  cancel(): void { this.editing.set(null); this.formOpen.set(false); this.form.controls.satellite_code.enable(); }
  save(): void {
    if (this.form.invalid) return; this.saving.set(true); const value = this.form.getRawValue(); const current = this.editing();
    const request = current ? this.api.updateSatellite(current.id, { name: value.name, orbit_class: value.orbit_class, supported_bands: value.supported_bands, priority_weight: value.priority_weight, minimum_contact_sec: value.minimum_contact_sec, asset_status: value.asset_status, expected_version: current.version }) : this.api.createSatellite(value as Omit<SatelliteAsset, 'id' | 'version' | 'updated_at'>);
    request.pipe(finalize(() => this.saving.set(false))).subscribe({ next: () => { this.snackBar.open(current ? 'Satellite updated' : 'Satellite created', 'Close', { duration: 2200 }); this.cancel(); this.load(); }, error: (error: unknown) => this.error.set(apiErrorMessage(error)) });
  }
}
