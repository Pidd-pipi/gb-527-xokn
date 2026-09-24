import { ChangeDetectionStrategy, Component, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { finalize } from 'rxjs';
import { useAuth } from '../hooks/use-auth';
import { apiErrorMessage } from '../hooks/use-conflict-detection';

@Component({
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule],
  template: `
    <main class="login-shell">
      <section class="identity">
        <span class="monogram">OD</span>
        <p>OFFLINE MISSION PLANNING</p>
        <h1>ORBIT DESK</h1>
        <div class="orbit-line"><span></span><i></i><b></b></div>
        <dl><div><dt>Mode</dt><dd>Planning only</dd></div><div><dt>Control link</dt><dd>Disconnected</dd></div><div><dt>Review</dt><dd>Required</dd></div></dl>
      </section>
      <section class="login-panel">
        <form [formGroup]="form" (ngSubmit)="submit()">
          <span class="eyebrow">Authenticated access</span>
          <h2>Planning console</h2>
          <p class="error" *ngIf="error()">{{ error() }}</p>
          <mat-form-field appearance="outline"><mat-label>Username</mat-label><input matInput formControlName="username" autocomplete="username"></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Password</mat-label><input matInput type="password" formControlName="password" autocomplete="current-password"></mat-form-field>
          <button mat-flat-button color="primary" type="submit" [disabled]="form.invalid || loading()">
            <span class="spinner" *ngIf="loading()"></span>{{ loading() ? 'Signing in' : 'Sign in' }}
          </button>
          <div class="accounts"><button type="button" (click)="account('scheduler')">Scheduler</button><button type="button" (click)="account('reviewer')">Reviewer</button><button type="button" (click)="account('admin')">Admin</button></div>
        </form>
      </section>
    </main>
  `,
  styles: [`
    .login-shell { min-height: 100vh; display: grid; grid-template-columns: minmax(320px, 1.05fr) minmax(360px, .95fr); background: #173f3b; }
    .identity { position: relative; overflow: hidden; display: flex; flex-direction: column; justify-content: center; padding: clamp(42px, 8vw, 120px); color: #f2f5ef; }
    .monogram { display: grid; place-items: center; width: 54px; height: 54px; background: #e7bd4f; color: #173f3b; font-weight: 900; }
    .identity p { margin: 42px 0 10px; color: #b9cec6; font-size: 11px; font-weight: 800; }
    .identity h1 { margin: 0; font-family: 'DIN Alternate', 'Avenir Next', sans-serif; font-size: 58px; line-height: 1; }
    .orbit-line { position: relative; width: min(520px, 90%); height: 88px; margin-top: 34px; border-top: 1px solid rgba(240,245,239,.28); transform: skewY(-5deg); }
    .orbit-line span, .orbit-line i, .orbit-line b { position: absolute; top: -5px; width: 9px; height: 9px; border-radius: 50%; background: #e7bd4f; }
    .orbit-line span { left: 8%; } .orbit-line i { left: 52%; background: #9fc6bb; } .orbit-line b { right: 5%; background: #f2f5ef; }
    dl { display: flex; gap: 34px; margin: 0; }
    dl div { min-width: 92px; } dt { color: #9eb8af; font-size: 9px; text-transform: uppercase; } dd { margin: 4px 0 0; font-size: 12px; }
    .login-panel { display: grid; place-items: center; padding: 32px; background: #f3f5f1; }
    form { width: min(410px, 100%); display: grid; }
    h2 { margin: 0 0 28px; font-size: 28px; }
    mat-form-field { width: 100%; }
    button[type=submit] { min-height: 46px; display: flex; gap: 9px; margin-top: 4px; }
    .accounts { display: flex; justify-content: center; gap: 18px; margin-top: 18px; }
    .accounts button { border: 0; background: transparent; color: #53615c; font-size: 11px; text-decoration: underline; cursor: pointer; }
    .error { padding: 10px; background: #f8e5e2; color: #7f302b; font-size: 12px; }
    @media (max-width: 760px) { .login-shell { grid-template-columns: 1fr; } .identity { min-height: 280px; padding: 38px 28px; } .identity h1 { font-size: 42px; } .orbit-line { height: 30px; } dl { margin-top: 12px; } .login-panel { min-height: 55vh; } }
  `],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class LoginPage {
  private readonly auth = useAuth();
  private readonly formBuilder = new FormBuilder().nonNullable;
  private readonly router: Router;
  readonly loading = signal(false);
  readonly error = signal('');
  readonly form = this.formBuilder.group({ username: ['scheduler', [Validators.required]], password: ['Scheduler#527', [Validators.required, Validators.minLength(8)]] });

  constructor(router: Router) { this.router = router; }

  account(name: 'scheduler' | 'reviewer' | 'admin'): void {
    const passwords = { scheduler: 'Scheduler#527', reviewer: 'Reviewer#527', admin: 'Admin#527' };
    this.form.setValue({ username: name, password: passwords[name] });
  }

  submit(): void {
    if (this.form.invalid) return;
    this.loading.set(true); this.error.set('');
    const { username, password } = this.form.getRawValue();
    this.auth.login(username, password).pipe(finalize(() => this.loading.set(false))).subscribe({
      next: () => void this.router.navigateByUrl('/windows'),
      error: (error: unknown) => this.error.set(apiErrorMessage(error)),
    });
  }
}
