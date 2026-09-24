import { ChangeDetectionStrategy, Component, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSnackBar, MatSnackBarModule } from '@angular/material/snack-bar';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { finalize } from 'rxjs';
import { ApiService } from '../api/api.service';
import { ConflictPreview, ConflictResolution, ConflictType, CONFLICT_TYPES, PreviewBlocker, PreviewWindowDisposition } from '../types/conflict';
import { ResolutionComparePanelComponent } from '../components/common/resolution-compare-panel.component';
import { WindowStatusBadgeComponent } from '../components/common/window-status-badge.component';
import { ConflictDetectionHook, apiErrorMessage, previewBlockers } from '../hooks/use-conflict-detection';
import { useAuth } from '../hooks/use-auth';
import { formatUtc, toLocalInput } from '../utils/date';

@Component({
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, MatSelectModule, MatSnackBarModule, MatProgressSpinnerModule, ResolutionComparePanelComponent, WindowStatusBadgeComponent],
  template: `
    <div class="page wide">
      <header class="page-head"><div><span class="eyebrow">Evidence and human decision</span><h1>Conflict resolution</h1><p>{{ resolutions().length }} recorded conflict groups</p></div></header>
      <div class="error-banner" *ngIf="error() || detection.error()"><span>{{ error() || detection.error() }}</span><button mat-button (click)="load()">Reload</button></div>
      <form class="detection-bar" [formGroup]="rangeForm" (ngSubmit)="detect()">
        <mat-form-field appearance="outline"><mat-label>Range start</mat-label><input matInput type="datetime-local" formControlName="from"></mat-form-field>
        <mat-form-field appearance="outline"><mat-label>Range end</mat-label><input matInput type="datetime-local" formControlName="to"></mat-form-field>
        <mat-form-field appearance="outline"><mat-label>Conflict type</mat-label><mat-select [value]="typeFilter()" (selectionChange)="filterType($event.value)"><mat-option value="">All types</mat-option><mat-option *ngFor="let type of types" [value]="type">{{ typeLabel(type) }}</mat-option></mat-select></mat-form-field>
        <button *ngIf="auth.canPlan()" mat-flat-button color="primary" type="submit" [disabled]="rangeForm.invalid || detection.running()"><span class="spinner" *ngIf="detection.running()"></span>{{ detection.running() ? 'Scanning' : 'Detect conflicts' }}</button>
      </form>
      <div class="detection-result" *ngIf="detection.result() as result"><strong>{{ result.conflict_count }} conflicts</strong><span>{{ result.window_count }} windows scanned</span><span>{{ format(result.range_from) }} – {{ format(result.range_to) }}</span></div>
      <div class="conflict-layout">
        <section class="conflict-list surface">
          <button *ngFor="let resolution of resolutions()" type="button" [class.active]="selected()?.id === resolution.id" (click)="select(resolution)">
            <span class="type-mark">{{ typeCode(resolution.conflict_type) }}</span><span><strong>{{ typeLabel(resolution.conflict_type) }}</strong><small>#{{ resolution.id }} · windows {{ resolution.window_ids.join(', ') }}</small></span><app-window-status-badge [status]="resolution.resolution_status" />
          </button><div class="empty" *ngIf="!resolutions().length">Run detection for the selected UTC range.</div>
        </section>
        <ng-container *ngIf="selected() as resolution; else noSelection">
          <section class="evidence surface">
            <div class="panel-head"><span class="eyebrow">Conflict evidence</span><h2>{{ typeLabel(resolution.conflict_type) }}</h2><app-window-status-badge [status]="resolution.resolution_status" /></div>
            <p class="summary">{{ resolution.evidence.summary }}</p>
            <dl><div><dt>Capacity</dt><dd>{{ resolution.evidence.capacity }}</dd></div><div><dt>Peak</dt><dd>{{ resolution.evidence.peak_concurrency }}</dd></div><div><dt>Buffer</dt><dd>{{ resolution.evidence.buffer_seconds }} sec</dd></div><div><dt>Version</dt><dd>v{{ resolution.version }}</dd></div></dl>
            <table><thead><tr><th>Window</th><th>Resource</th><th>Interval</th><th>Band</th><th>Lock</th></tr></thead><tbody><tr *ngFor="let fact of resolution.evidence.window_facts"><td class="code">#{{ fact['id'] }}</td><td>S{{ fact['station_id'] }} / A{{ fact['satellite_id'] }}</td><td>{{ formatFact(fact['start_at']) }}<br><span>{{ fact['duration_sec'] }} sec</span></td><td>{{ fact['band'] }}</td><td>{{ fact['locked'] ? 'locked' : 'open' }}</td></tr></tbody></table>
            <details><summary>Detection metadata</summary><pre>{{ resolution.evidence.metadata | json }}</pre></details>
          </section>
          <section class="decision">
            <div class="section-title"><h2>Ranked options</h2><span>Stable score order</span></div>
            <app-resolution-compare-panel [resolution]="resolution" [selectedKey]="selectedKey()" [readonly]="resolution.resolution_status === 'accepted' || resolution.resolution_status === 'rejected'" (selectedKeyChange)="chooseAction($event)" />
            <div class="workflow surface">
              <ng-container *ngIf="resolution.resolution_status === 'proposed' && auth.canPlan()"><p>Submit this evidence set and its ranked options for independent review.</p><button mat-flat-button color="primary" (click)="submitForReview(resolution)">Submit for review</button></ng-container>
              <form *ngIf="resolution.resolution_status === 'pending_review' && auth.canReview()" [formGroup]="reviewForm">
                <div class="preview-bar">
                  <button mat-stroked-button type="button" [disabled]="!selectedKey() || previewRunning()" (click)="preview(resolution)">
                    <mat-spinner *ngIf="previewRunning()" diameter="16"></mat-spinner>Run read-only preview
                  </button>
                  <span class="muted">Computed against current window versions; nothing is changed.</span>
                </div>
                <section class="preview-result" *ngIf="previewForId() === resolution.id && previewResult() as result">
                  <div class="preview-head"><strong>Preview · {{ actionTypeLabel(result.action_type) }}</strong><span class="readonly-tag">read-only</span><span [class.flagged]="result.remaining_conflict_count > 0">{{ result.remaining_conflict_count }} conflict(s) would remain</span></div>
                  <ul class="dispositions">
                    <li *ngFor="let item of result.window_dispositions" [class]="'disp-' + item.disposition"><i>{{ dispositionLabel(item) }}</i><strong>Window #{{ item.window_id }}<span *ngIf="item.target_station_code"> → {{ item.target_station_code }}</span><span *ngIf="item.alternate_window_id"> → #{{ item.alternate_window_id }}</span></strong><small>{{ item.note }}</small></li>
                  </ul>
                  <div class="remaining" *ngIf="result.remaining_conflicts.length">
                    <h4>Remaining conflicts</h4>
                    <p *ngFor="let conflict of result.remaining_conflicts"><i>{{ typeLabel(conflict.conflict_type) }}</i> windows {{ conflict.window_ids.join(', ') }} — {{ conflict.summary }}</p>
                  </div>
                  <p class="remaining clear" *ngIf="!result.remaining_conflicts.length">No conflict remains in the simulated plan for the referenced windows.</p>
                  <p class="manual-hint" *ngIf="result.requires_manual">This option keeps the case open for an operator-authored decision.</p>
                </section>
                <section class="preview-blocked" *ngIf="previewForId() === resolution.id && previewBlockers().length">
                  <h4>Preview blocked — planning data changed (409)</h4>
                  <p *ngFor="let blocker of previewBlockers()"><i>{{ blocker.code }}</i> {{ blocker.message }}</p>
                </section>
                <mat-form-field appearance="outline"><mat-label>Review note</mat-label><textarea matInput rows="3" formControlName="note"></textarea></mat-form-field><div class="action-row"><button mat-stroked-button color="warn" type="button" (click)="review(resolution, 'rejected')">Reject</button><button mat-flat-button color="primary" type="button" [disabled]="!selectedKey()" (click)="review(resolution, 'accepted')">Accept selected</button></div></form>
              <div *ngIf="resolution.resolution_status === 'accepted' || resolution.resolution_status === 'rejected'"><strong>Decision {{ resolution.resolution_status }}</strong><p>{{ resolution.review_note || 'No review note recorded.' }}</p><span class="muted">{{ resolution.resolved_by }} · {{ resolution.resolved_at ? format(resolution.resolved_at) : '' }}</span></div>
              <p *ngIf="resolution.resolution_status === 'pending_review' && !auth.canReview()">Pending reviewer action.</p>
            </div>
          </section>
        </ng-container>
        <ng-template #noSelection><div class="empty surface">Select a conflict group to inspect evidence and suggestions.</div></ng-template>
      </div>
    </div>
  `,
  styles: [`
    .wide { width: min(1600px, 100%); }
    .detection-bar { display: grid; grid-template-columns: repeat(3, minmax(180px, 1fr)) auto; gap: 12px; align-items: start; padding: 15px; background: #e9eeea; border: 1px solid #ccd3ce; } .detection-bar button { min-height: 48px; display: flex; gap: 8px; }
    .detection-result { display: flex; flex-wrap: wrap; gap: 18px; padding: 10px 14px; background: #173f3b; color: #eef4ef; font-size: 11px; } .detection-result span { color: #bad0c7; }
    .conflict-layout { display: grid; grid-template-columns: minmax(245px, .65fr) minmax(330px, 1fr) minmax(380px, 1.15fr); gap: 16px; align-items: start; margin-top: 20px; }
    .conflict-list { max-height: 720px; overflow-y: auto; } .conflict-list > button { width: 100%; display: grid; grid-template-columns: 34px minmax(0, 1fr) auto; align-items: center; gap: 10px; padding: 13px; border: 0; border-bottom: 1px solid #e0e5e1; background: transparent; text-align: left; cursor: pointer; } .conflict-list > button:hover, .conflict-list > button.active { background: #edf3ef; } .conflict-list strong, .conflict-list small { display: block; } .conflict-list strong { font-size: 12px; } .conflict-list small { margin-top: 3px; color: #66716d; font-size: 9px; } .type-mark { display: grid; place-items: center; width: 31px; height: 31px; background: #dfe7e2; font-size: 9px; font-weight: 900; }
    .evidence { overflow: hidden; } .panel-head { display: grid; grid-template-columns: 1fr auto; align-items: center; padding: 18px; border-bottom: 1px solid #ccd3ce; } .panel-head .eyebrow { grid-column: 1/-1; } .panel-head h2 { margin: 0; font-size: 17px; } .summary { margin: 0; padding: 16px 18px; line-height: 1.5; font-size: 13px; } dl { display: grid; grid-template-columns: repeat(4, 1fr); margin: 0; border-block: 1px solid #dfe4e0; } dl div { padding: 11px; border-right: 1px solid #dfe4e0; } dt { color: #66716d; font-size: 9px; text-transform: uppercase; } dd { margin: 3px 0 0; font-weight: 800; } table { width: 100%; border-collapse: collapse; } th, td { padding: 9px; border-bottom: 1px solid #e2e7e3; text-align: left; font-size: 10px; } th { color: #66716d; } td span { color: #66716d; } details { padding: 12px 16px; } summary { cursor: pointer; font-size: 11px; } pre { max-width: 100%; overflow: auto; font-size: 10px; white-space: pre-wrap; }
    .decision .section-title { margin-top: 0; } .workflow { margin-top: 12px; padding: 16px; } .workflow p { color: #66716d; font-size: 12px; line-height: 1.5; } .workflow form { display: grid; } .workflow mat-form-field { width: 100%; }
    .preview-bar { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; } .preview-bar button { display: inline-flex; align-items: center; gap: 7px; } .preview-bar .muted { font-size: 10px; color: #66716d; }
    .preview-result { border: 1px solid #ccd3ce; background: #fbfcf8; padding: 12px 14px; margin-bottom: 14px; } .preview-head { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; font-size: 12px; } .preview-head .readonly-tag { padding: 2px 7px; background: #173f3b; color: #eef4ef; font-size: 9px; text-transform: uppercase; } .preview-head span:last-child { margin-left: auto; font-size: 11px; color: #1c625b; font-weight: 700; } .preview-head span.flagged { color: #a02d22; }
    .dispositions { list-style: none; margin: 12px 0 0; padding: 0; display: grid; gap: 8px; } .dispositions li { display: grid; grid-template-columns: 150px 1fr; gap: 4px 12px; padding: 8px 10px; border-left: 4px solid #99a79f; background: #f1f4f1; } .dispositions li i { font-style: normal; font-size: 9px; text-transform: uppercase; font-weight: 800; color: #4b5a53; align-self: center; } .dispositions li strong { font-size: 12px; } .dispositions li small { grid-column: 2; color: #66716d; font-size: 10px; } .disp-keep { border-left-color: #1c625b !important; } .disp-reassign { border-left-color: #2b5d8a !important; } .disp-use_alternate_window { border-left-color: #7a5d12 !important; } .disp-manual { border-left-color: #a02d22 !important; }
    .remaining { margin-top: 12px; border-top: 1px dashed #ccd3ce; padding-top: 10px; } .remaining h4, .preview-blocked h4 { margin: 0 0 6px; font-size: 11px; text-transform: uppercase; color: #4b5a53; } .remaining p { margin: 4px 0; font-size: 11px; display: flex; gap: 8px; } .remaining p i { font-style: normal; font-weight: 800; color: #a02d22; min-width: 120px; } .remaining.clear { color: #1c625b; } .manual-hint { margin-top: 8px; font-size: 10px; color: #7a5d12; }
    .preview-blocked { border: 1px solid #d8a29a; background: #fbeeec; padding: 11px 13px; margin-bottom: 14px; } .preview-blocked p { margin: 4px 0; font-size: 11px; } .preview-blocked p i { font-style: normal; font-weight: 800; color: #a02d22; margin-right: 8px; }
    @media (max-width: 1220px) { .conflict-layout { grid-template-columns: 280px 1fr; } .decision { grid-column: 2; } } @media (max-width: 800px) { .detection-bar { grid-template-columns: 1fr 1fr; } .conflict-layout { grid-template-columns: 1fr; } .decision { grid-column: 1; } }
  `],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ConflictsPage implements OnInit {
  readonly auth = useAuth(); readonly detection: ConflictDetectionHook; readonly resolutions = signal<ConflictResolution[]>([]); readonly selected = signal<ConflictResolution | null>(null); readonly selectedKey = signal(''); readonly error = signal(''); readonly typeFilter = signal(''); readonly types = CONFLICT_TYPES;
  readonly previewRunning = signal(false); readonly previewResult = signal<ConflictPreview | null>(null); readonly previewForId = signal(0); readonly previewBlockers = signal<PreviewBlocker[]>([]);
  private readonly builder = new FormBuilder().nonNullable;
  readonly rangeForm = this.builder.group({ from: ['', Validators.required], to: ['', Validators.required] }); readonly reviewForm = this.builder.group({ note: ['', Validators.maxLength(500)] });
  constructor(private readonly api: ApiService, detection: ConflictDetectionHook, private readonly snackBar: MatSnackBar) { this.detection = detection; const from = new Date(Date.now() - 60 * 60 * 1000), to = new Date(Date.now() + 26 * 60 * 60 * 1000); this.rangeForm.setValue({ from: toLocalInput(from), to: toLocalInput(to) }); }
  ngOnInit(): void { this.load(); }
  load(preferredId = this.selected()?.id ?? 0): void { this.error.set(''); const params: Record<string, string | number> = { page_size: 100 }; if (this.typeFilter()) params['conflict_type'] = this.typeFilter(); this.api.conflicts(params).subscribe({ next: (response) => { this.resolutions.set(response.data); const next = response.data.find((item) => item.id === preferredId) ?? response.data[0] ?? null; this.select(next); }, error: (error: unknown) => this.error.set(apiErrorMessage(error)) }); }
  filterType(value: string): void { this.typeFilter.set(value); this.load(0); }
  select(resolution: ConflictResolution | null): void { this.selected.set(resolution); this.selectedKey.set(resolution?.selected_action?.action_key ?? resolution?.suggestions[0]?.action_key ?? ''); this.resetPreview(); }
  detect(): void { if (this.rangeForm.invalid) return; const range = this.rangeForm.getRawValue(); this.detection.detect(new Date(range.from).toISOString(), new Date(range.to).toISOString(), () => { this.snackBar.open('Conflict scan completed', 'Close', { duration: 2200 }); this.load(0); }); }
  submitForReview(resolution: ConflictResolution): void { this.api.submitConflict(resolution.id, resolution.version).subscribe({ next: (response) => { this.snackBar.open('Submitted for review', 'Close', { duration: 2200 }); this.load(response.data.id); }, error: (error: unknown) => this.error.set(apiErrorMessage(error)) }); }
  review(resolution: ConflictResolution, decision: 'accepted' | 'rejected'): void { const key = decision === 'accepted' ? this.selectedKey() : ''; this.api.reviewConflict(resolution.id, resolution.version, decision, key, this.reviewForm.controls.note.value).pipe(finalize(() => undefined)).subscribe({ next: (response) => { this.snackBar.open(`Resolution ${decision}`, 'Close', { duration: 2200 }); this.load(response.data.id); }, error: (error: unknown) => this.error.set(apiErrorMessage(error)) }); }
  preview(resolution: ConflictResolution): void {
    const actionKey = this.selectedKey();
    if (!actionKey || this.previewRunning()) return;
    this.previewRunning.set(true); this.previewResult.set(null); this.previewBlockers.set([]); this.previewForId.set(resolution.id);
    this.api.previewConflict(resolution.id, resolution.version, actionKey).pipe(finalize(() => this.previewRunning.set(false))).subscribe({
      next: (response) => this.previewResult.set(response.data),
      error: (error: unknown) => this.previewBlockers.set(previewBlockers(error)),
    });
  }
  resetPreview(): void { this.previewResult.set(null); this.previewBlockers.set([]); this.previewForId.set(0); }
  chooseAction(actionKey: string): void { this.selectedKey.set(actionKey); this.resetPreview(); }
  dispositionLabel(item: PreviewWindowDisposition): string { return item.disposition_label; }
  actionTypeLabel(value: string): string { return value.replaceAll('_', ' '); }
  typeLabel(value: string): string { return value.replaceAll('_', ' '); } typeCode(value: ConflictType): string { return ({ station_capacity: 'SC', satellite_overlap: 'SO', band_mismatch: 'BM', duration_shortfall: 'DS', slew_buffer: 'SB' })[value]; }
  format(value: string): string { return formatUtc(value); } formatFact(value: unknown): string { return typeof value === 'string' ? formatUtc(value) : '—'; }
}
