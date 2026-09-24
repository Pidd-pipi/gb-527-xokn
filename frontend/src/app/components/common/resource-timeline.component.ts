import { ChangeDetectionStrategy, Component, Input } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ContactWindow } from '../../types/window';
import { WindowStatusBadgeComponent } from './window-status-badge.component';
import { formatUtc } from '../../utils/date';

@Component({
  selector: 'app-resource-timeline',
  standalone: true,
  imports: [CommonModule, WindowStatusBadgeComponent],
  template: `
    <div class="timeline" *ngIf="windows.length; else noData">
      <div class="axis"><span>{{ format(start) }}</span><span>{{ format(end) }}</span></div>
      <div class="lane" *ngFor="let lane of lanes">
        <div class="lane-label"><strong>{{ lane }}</strong><span>{{ laneWindows(lane).length }} windows</span></div>
        <div class="track">
          <button *ngFor="let window of laneWindows(lane)" type="button" class="window"
            [class.locked]="window.locked" [style.left.%]="left(window)" [style.width.%]="width(window)"
            [attr.title]="window.satellite_code + ' · ' + format(window.start_at) + '–' + format(window.end_at)"
            (click)="select(window)">
            <span>{{ label(window) }}</span><small>{{ window.band }} · P{{ window.priority }}</small>
          </button>
        </div>
      </div>
      <div class="selection" *ngIf="selected">
        <strong>#{{ selected.id }} {{ selected.satellite_code }}</strong>
        <span>{{ selected.station_code }} · {{ format(selected.start_at) }} – {{ format(selected.end_at) }}</span>
        <app-window-status-badge [status]="selected.window_status" />
      </div>
    </div>
    <ng-template #noData><div class="empty">No contact windows match this range.</div></ng-template>
  `,
  styleUrl: './resource-timeline.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ResourceTimelineComponent {
  @Input({ required: true }) windows: ContactWindow[] = [];
  @Input() groupBy: 'station' | 'satellite' = 'station';
  selected: ContactWindow | null = null;

  get start(): string { return this.windows.reduce((value, item) => item.start_at < value ? item.start_at : value, this.windows[0]?.start_at ?? new Date().toISOString()); }
  get end(): string { return this.windows.reduce((value, item) => item.end_at > value ? item.end_at : value, this.windows[0]?.end_at ?? new Date().toISOString()); }
  get lanes(): string[] { return [...new Set(this.windows.map((item) => this.groupBy === 'station' ? item.station_code : item.satellite_code))].sort(); }
  laneWindows(lane: string): ContactWindow[] { return this.windows.filter((item) => (this.groupBy === 'station' ? item.station_code : item.satellite_code) === lane); }
  label(window: ContactWindow): string { return this.groupBy === 'station' ? window.satellite_code : window.station_code; }
  format(value: string): string { return formatUtc(value); }
  select(window: ContactWindow): void { this.selected = window; }
  left(window: ContactWindow): number { return this.position(window.start_at); }
  width(window: ContactWindow): number { return Math.max(4, this.position(window.end_at) - this.position(window.start_at)); }
  private position(value: string): number {
    const start = new Date(this.start).getTime(), end = new Date(this.end).getTime();
    if (end <= start) return 0;
    return Math.max(0, Math.min(100, ((new Date(value).getTime() - start) / (end - start)) * 100));
  }
}
