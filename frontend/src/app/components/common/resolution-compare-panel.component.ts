import { ChangeDetectionStrategy, Component, EventEmitter, Input, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ConflictResolution, ResolutionSuggestion } from '../../types/conflict';

@Component({
  selector: 'app-resolution-compare-panel',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="compare" *ngIf="resolution">
      <button *ngFor="let suggestion of resolution.suggestions; let index = index" type="button" class="suggestion"
        [class.selected]="selectedKey === suggestion.action_key" [disabled]="readonly" (click)="choose(suggestion)">
        <span class="rank">{{ index + 1 }}</span>
        <span class="body"><strong>{{ suggestion.title }}</strong><small>{{ suggestion.rationale }}</small>
          <span class="tags"><i>score {{ suggestion.score.total_score | number:'1.2-2' }}</i><i>loss {{ suggestion.score.priority_loss }}</i><i>{{ suggestion.score.contact_duration_sec }} sec</i><i *ngIf="suggestion.requires_manual">manual</i></span>
        </span>
        <span class="choice">{{ selectedKey === suggestion.action_key ? 'SELECTED' : 'SELECT' }}</span>
      </button>
    </div>
  `,
  styles: [`
    .compare { display: grid; gap: 8px; }
    .suggestion { width: 100%; min-height: 92px; display: grid; grid-template-columns: 30px 1fr auto; gap: 12px; align-items: start; padding: 13px; border: 1px solid #ccd3ce; border-radius: 3px; background: #fbfcf8; color: #18211f; text-align: left; cursor: pointer; }
    .suggestion:hover:not(:disabled) { border-color: #6e8c83; background: #f1f6f2; }
    .suggestion.selected { border-color: #1c625b; box-shadow: inset 3px 0 #1c625b; }
    .suggestion:disabled { cursor: default; opacity: 1; }
    .rank { display: grid; place-items: center; width: 26px; height: 26px; background: #e6ebe7; color: #54615d; font-size: 11px; font-weight: 800; }
    .body strong, .body small { display: block; }
    .body strong { font-size: 13px; }
    .body small { margin-top: 5px; color: #66716d; font-size: 11px; line-height: 1.45; }
    .tags { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 9px; }
    .tags i { padding: 2px 6px; background: #e9eeea; font-size: 9px; font-style: normal; text-transform: uppercase; }
    .choice { align-self: center; color: #1c625b; font-size: 9px; font-weight: 800; }
    @media (max-width: 560px) { .suggestion { grid-template-columns: 26px 1fr; } .choice { display: none; } }
  `],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ResolutionComparePanelComponent {
  @Input({ required: true }) resolution: ConflictResolution | null = null;
  @Input() selectedKey = '';
  @Input() readonly = false;
  @Output() selectedKeyChange = new EventEmitter<string>();
  choose(suggestion: ResolutionSuggestion): void { if (!this.readonly) this.selectedKeyChange.emit(suggestion.action_key); }
}
