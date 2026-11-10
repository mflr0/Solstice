/** The guided tour of the app shell: its steps, for whom, with which keys. */

import { bindingWorks, type Shortcut, type ShortcutAction } from './shortcuts.js';
import type { Preferences } from './types.js';

/** A step of the tour, in the order it shows them. */
export type TourStepId =
  | 'welcome'
  | 'scope'
  | 'work'
  | 'projects'
  | 'boards'
  | 'quickSearch'
  | 'create'
  | 'themes'
  | 'account'
  | 'admin';

/** The `data-tour` value of a top-bar element a step points at. */
export type TourTarget =
  'scope' | 'work' | 'projects' | 'boards' | 'quick-search' | 'create' | 'themes' | 'account';

/** A key the step names, for an action that has one that works. */
export interface TourKey {
  action: ShortcutAction;
  binding: string;
}

export interface TourStep {
  id: TourStepId;
  /** Null: a centered card. */
  target: TourTarget | null;
  keys: TourKey[];
}

export interface TourOptions {
  admin: boolean;
  /** Whether single-key shortcuts are on. */
  shortcuts: boolean;
  bindings: readonly Shortcut[];
  mac: boolean;
}

/** The first binding of a global action that works, or null. */
export function workingKey(
  action: ShortcutAction,
  o: Pick<TourOptions, 'shortcuts' | 'bindings' | 'mac'>,
): TourKey | null {
  const binding = o.bindings
    .find((s) => s.scope === 'global' && s.action === action)
    ?.keys.find((k) => bindingWorks(k, o.shortcuts, o.mac));
  return binding ? { action, binding } : null;
}

/** The steps for this user, in order. */
export function tourSteps(o: TourOptions): TourStep[] {
  const keys = (...actions: ShortcutAction[]) =>
    actions.map((a) => workingKey(a, o)).filter((k): k is TourKey => k !== null);
  const steps: TourStep[] = [
    { id: 'welcome', target: null, keys: [] },
    { id: 'scope', target: 'scope', keys: [] },
    { id: 'work', target: 'work', keys: [] },
    { id: 'projects', target: 'projects', keys: [] },
    { id: 'boards', target: 'boards', keys: [] },
    { id: 'quickSearch', target: 'quick-search', keys: keys('search') },
    { id: 'create', target: 'create', keys: keys('create', 'help') },
    { id: 'themes', target: 'themes', keys: [] },
    { id: 'account', target: 'account', keys: [] },
  ];
  if (o.admin) steps.push({ id: 'admin', target: 'account', keys: [] });
  return steps;
}

/** Whether the tour starts by itself: the account has not seen it yet. */
export function startsTour(preferences: Preferences | undefined): boolean {
  return preferences?.tourSeen !== true;
}
