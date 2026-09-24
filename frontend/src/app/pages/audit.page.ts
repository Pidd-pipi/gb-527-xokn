import { ChangeDetectionStrategy, Component, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatSelectModule } from '@angular/material/select';
import { forkJoin } from 'rxjs';
import { ApiService } from '../api/api.service';
import { AuditEvent, SatelliteAsset } from '../types/resources';
import { ConflictResolution } from '../types/conflict';
import { ResolutionComparePanelComponent } from '../components/common/resolution-compare-panel.component';
import { WindowStatusBadgeComponent } from '../components/common/window-status-badge.component';
import { apiErrorMessage } from '../hooks/use-conflict-detection';
import { formatUtc } from '../utils/date';

@Component({
  standalone: true,
  imports: [CommonModule, MatButtonModule, MatFormFieldModule, MatSelectModule, ResolutionComparePanelComponent, WindowStatusBadgeComponent],
  template: `
    <div class="page">
      <header class="page-head"><div><span class="eyebrow">Version and decision ledger</span><h1>Planning audit</h1><p>{{ events().length }} recent events · {{ satellites().length }} satellite inputs</p></div><button mat-stroked-button (click)="load()">Refresh</button></header>
      <div class="error-banner" *ngIf="error()"><span>{{ error() }}</span><button mat-button (click)="load()">Retry</button></div>
      <div class="metric-strip"><div class="metric"><strong>{{ events().length }}</strong><span>events loaded</span></div><div class="metric"><strong>{{ decisionCount() }}</strong><span>human decisions</span></div><div class="metric"><strong>{{ latestAssetVersion() }}</strong><span>max asset version</span></div></div>
      <div class="audit-grid">
        <section>
          <div class="section-title"><h2>Operation ledger</h2><span>Request-linked summaries</span></div>
          <div class="surface event-list"><button *ngFor="let event of events()" [class.active]="selectedEvent()?.id === event.id" (click)="selectedEvent.set(event)"><span class="event-code">{{ actionCode(event.action) }}</span><span><strong>{{ event.action }}</strong><small>{{ event.actor }} · {{ format(event.created_at) }}</small></span><span class="code">{{ event.request_id.slice(0, 8) }}</span></button></div>
        </section>
        <section>
          <div class="section-title"><h2>Version comparison</h2><span>{{ selectedEvent()?.resource_type || 'Select an event' }}</span></div>
          <div class="surface diff" *ngIf="selectedEvent() as event; else emptyDiff"><header><strong>{{ event.action }}</strong><span class="code">{{ event.resource_type }} #{{ event.resource_id }}</span></header><div class="diff-columns"><article><h3>Before</h3><pre>{{ event.before_summary | json }}</pre></article><article><h3>After</h3><pre>{{ event.after_summary | json }}</pre></article></div><footer><span>Parameters</span><pre>{{ event.parameters | json }}</pre></footer></div>
          <ng-template #emptyDiff><div class="surface empty">Select an audit event to compare its before and after summaries.</div></ng-template>
        </section>
      </div>
      <div class="section-title"><h2>Resolution decisions</h2><span>Accepted choice remains immutable</span></div>
      <div class="resolution-band"><div class="surface resolution-index"><button *ngFor="let resolution of resolutions()" [class.active]="selectedResolution()?.id === resolution.id" (click)="selectedResolution.set(resolution)"><span>#{{ resolution.id }} {{ resolution.conflict_type.replaceAll('_', ' ') }}</span><app-window-status-badge [status]="resolution.resolution_status" /></button></div>
        <div class="surface resolution-view" *ngIf="selectedResolution() as resolution"><header><strong>Conflict #{{ resolution.id }}</strong><span>{{ resolution.resolved_by || 'Unresolved' }}</span></header><app-resolution-compare-panel [resolution]="resolution" [selectedKey]="resolution.selected_action?.action_key || ''" [readonly]="true" /></div></div>
      <div class="section-title"><h2>Satellite input versions</h2><span>Planning assets referenced by this audit view</span></div><div class="surface asset-strip"><div *ngFor="let asset of satellites()"><strong>{{ asset.satellite_code }}</strong><span>v{{ asset.version }} · weight {{ asset.priority_weight }} · {{ asset.minimum_contact_sec }} sec</span></div></div>
    </div>
  `,
  styles: [`
    .audit-grid { display: grid; grid-template-columns: minmax(320px, .9fr) minmax(420px, 1.1fr); gap: 18px; } .event-list { max-height: 520px; overflow-y: auto; } .event-list button { width: 100%; display: grid; grid-template-columns: 34px 1fr auto; gap: 10px; align-items: center; padding: 11px 13px; border: 0; border-bottom: 1px solid #e1e6e2; background: transparent; text-align: left; cursor: pointer; } .event-list button:hover, .event-list button.active { background: #edf3ef; } .event-list strong, .event-list small { display: block; } .event-list strong { font-size: 12px; } .event-list small { margin-top: 3px; color: #66716d; font-size: 10px; } .event-code { display: grid; place-items: center; width: 30px; height: 30px; background: #dfe7e2; color: #28584f; font-size: 9px; font-weight: 900; }
    .diff header, .diff footer { display: flex; justify-content: space-between; gap: 12px; padding: 13px 15px; border-bottom: 1px solid #dfe4e0; font-size: 12px; } .diff-columns { display: grid; grid-template-columns: 1fr 1fr; } .diff article { min-width: 0; padding: 14px; } .diff article:first-child { border-right: 1px solid #dfe4e0; } .diff h3 { margin: 0 0 8px; color: #66716d; font-size: 9px; text-transform: uppercase; } .diff pre { min-height: 130px; margin: 0; overflow: auto; white-space: pre-wrap; font-size: 10px; line-height: 1.45; } .diff footer { display: block; border-top: 1px solid #dfe4e0; border-bottom: 0; } .diff footer pre { min-height: 0; margin: 8px 0 0; }
    .resolution-band { display: grid; grid-template-columns: 320px minmax(0, 1fr); gap: 18px; align-items: start; } .resolution-index button { width: 100%; min-height: 48px; display: flex; justify-content: space-between; align-items: center; gap: 10px; padding: 9px 12px; border: 0; border-bottom: 1px solid #e1e6e2; background: transparent; text-align: left; cursor: pointer; } .resolution-index button.active { background: #edf3ef; } .resolution-index span { font-size: 11px; } .resolution-view { padding: 14px; } .resolution-view header { display: flex; justify-content: space-between; margin-bottom: 12px; font-size: 12px; }
    .asset-strip { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); } .asset-strip div { padding: 14px; border-right: 1px solid #e1e6e2; } .asset-strip strong, .asset-strip span { display: block; } .asset-strip span { margin-top: 4px; color: #66716d; font-size: 10px; }
    @media (max-width: 980px) { .audit-grid, .resolution-band { grid-template-columns: 1fr; } } @media (max-width: 560px) { .diff-columns { grid-template-columns: 1fr; } .diff article:first-child { border-right: 0; border-bottom: 1px solid #dfe4e0; } }
  `],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AuditPage implements OnInit {
  readonly events = signal<AuditEvent[]>([]); readonly satellites = signal<SatelliteAsset[]>([]); readonly resolutions = signal<ConflictResolution[]>([]); readonly selectedEvent = signal<AuditEvent | null>(null); readonly selectedResolution = signal<ConflictResolution | null>(null); readonly error = signal('');
  constructor(private readonly api: ApiService) {}
  ngOnInit(): void { this.load(); }
  load(): void { this.error.set(''); forkJoin([this.api.audit({ page_size: 100 }), this.api.satellites({ page_size: 100 }), this.api.conflicts({ page_size: 100 })]).subscribe({ next: ([events, satellites, resolutions]) => { this.events.set(events.data); this.satellites.set(satellites.data); this.resolutions.set(resolutions.data); this.selectedEvent.set(events.data[0] ?? null); this.selectedResolution.set(resolutions.data.find((item) => item.resolution_status === 'accepted') ?? resolutions.data[0] ?? null); }, error: (error: unknown) => this.error.set(apiErrorMessage(error)) }); }
  decisionCount(): number { return this.events().filter((event) => event.action === 'conflict.reviewed').length; }
  latestAssetVersion(): number { return Math.max(0, ...this.satellites().map((asset) => asset.version)); }
  actionCode(action: string): string { return action.split('.').map((value) => value[0]?.toUpperCase() ?? '').join('').slice(0, 2); }
  format(value: string): string { return formatUtc(value); }
}
