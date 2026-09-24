import { Injectable, inject, signal } from '@angular/core';
import { finalize, switchMap, timer } from 'rxjs';
import { ApiService } from '../api/api.service';
import { DetectionResult } from '../types/conflict';

@Injectable({ providedIn: 'root' })
export class ConflictDetectionHook {
  private readonly api = inject(ApiService);
  readonly running = signal(false);
  readonly result = signal<DetectionResult | null>(null);
  readonly error = signal('');

  detect(from: string, to: string, completed: () => void): void {
    this.running.set(true);
    this.error.set('');
    this.api.detectConflicts(from, to).pipe(
      switchMap((response) => {
        this.result.set(response.data);
        return timer(350).pipe(switchMap(() => this.api.conflicts({ page_size: 100 })));
      }),
      finalize(() => this.running.set(false)),
    ).subscribe({
      next: () => completed(),
      error: (error: unknown) => this.error.set(apiErrorMessage(error)),
    });
  }
}

export function apiErrorMessage(error: unknown): string {
  if (typeof error === 'object' && error !== null) {
    const body = (error as { error?: { error?: { message?: string } } }).error;
    return body?.error?.message ?? 'Request failed';
  }
  return 'Request failed';
}
