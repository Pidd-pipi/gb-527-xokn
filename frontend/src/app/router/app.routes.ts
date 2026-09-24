import { Routes, CanActivateFn, Router } from '@angular/router';
import { inject } from '@angular/core';
import { AuthStore } from '../stores/auth.store';

const authGuard: CanActivateFn = () => {
  const auth = inject(AuthStore);
  return auth.authenticated() ? true : inject(Router).createUrlTree(['/login']);
};

const guestGuard: CanActivateFn = () => {
  const auth = inject(AuthStore);
  return auth.authenticated() ? inject(Router).createUrlTree(['/windows']) : true;
};

const reviewGuard: CanActivateFn = () => {
  const auth = inject(AuthStore);
  return auth.canReview() ? true : inject(Router).createUrlTree(['/conflicts']);
};

export const routes: Routes = [
  { path: 'login', canActivate: [guestGuard], loadComponent: () => import('../pages/login.page').then((module) => module.LoginPage) },
  { path: 'stations', canActivate: [authGuard], loadComponent: () => import('../pages/stations.page').then((module) => module.StationsPage) },
  { path: 'satellites', canActivate: [authGuard], loadComponent: () => import('../pages/satellites.page').then((module) => module.SatellitesPage) },
  { path: 'windows', canActivate: [authGuard], loadComponent: () => import('../pages/windows.page').then((module) => module.WindowsPage) },
  { path: 'conflicts', canActivate: [authGuard], loadComponent: () => import('../pages/conflicts.page').then((module) => module.ConflictsPage) },
  { path: 'audit', canActivate: [authGuard, reviewGuard], loadComponent: () => import('../pages/audit.page').then((module) => module.AuditPage) },
  { path: '', pathMatch: 'full', redirectTo: 'windows' },
  { path: '**', redirectTo: 'windows' },
];
