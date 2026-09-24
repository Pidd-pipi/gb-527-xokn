import { ChangeDetectionStrategy, Component, Input } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'app-window-status-badge',
  standalone: true,
  imports: [CommonModule],
  template: `<span class="status" [ngClass]="status">{{ label }}</span>`,
  styles: [`
    .status { display: inline-flex; align-items: center; min-height: 24px; padding: 2px 8px; border: 1px solid #c9d1cc; border-radius: 999px; background: #eef1ee; color: #43504c; font-size: 10px; font-weight: 800; text-transform: uppercase; white-space: nowrap; }
    .candidate, .detected { background: #edf0eb; }
    .submitted, .proposed { background: #e6eef5; color: #294f72; border-color: #b9cbd9; }
    .locked, .pending_review { background: #fff2c7; color: #73550a; border-color: #dec778; }
    .allocated, .accepted { background: #e0efe7; color: #205d45; border-color: #afd0be; }
    .cancelled, .rejected { background: #f6e2df; color: #8a352f; border-color: #dcaaa5; }
  `],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WindowStatusBadgeComponent {
  @Input({ required: true }) status = '';
  get label(): string { return this.status.replaceAll('_', ' '); }
}
