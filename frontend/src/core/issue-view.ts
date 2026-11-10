// The issue view's fields, as its layout places them.

import type { LayoutField } from './admin-types.js';
import type { CustomValue } from './types.js';

/**
 * The view layout when the server's can't be read: the built-in default
 * (backend/internal/layout, defaults).
 */
export const defaultViewLayout: readonly LayoutField[] = [
  ...['description', 'environment'].map((field) => ({ field, zone: 'main' as const })),
  ...[
    ...['assignee', 'reporter', 'priority', 'components', 'fixVersions', 'versions', 'epic'],
    ...['sprint', 'labels', 'duedate', 'security', 'created', 'updated', 'resolutiondate'],
    'timetracking',
  ].map((field) => ({ field, zone: 'side' as const })),
].map((f) => ({ ...f, hidden: false }));

/** A custom field's layout id. */
export const customFieldId = (id: number) => `customfield_${id}`;

/** The custom field a layout id names, 0 for a system field. */
export function customOf(field: string): number {
  const m = /^customfield_([1-9]\d*)$/.exec(field);
  return m ? Number(m[1]) : 0;
}

/** The view's zones, in order, and the custom values its layout doesn't place. */
export interface ViewZones {
  main: string[];
  side: string[];
  unplaced: CustomValue[];
}

/**
 * Splits a view layout into its shown fields by zone. A hidden field shows
 * nowhere; a value of a custom field the layout leaves out (no longer
 * offered) goes after the side card's fields, by name.
 */
export function viewZones(
  layout: readonly LayoutField[],
  values: readonly CustomValue[],
): ViewZones {
  const shown = layout.filter((f) => !f.hidden);
  const placed = new Set(layout.map((f) => f.field));
  return {
    main: shown.filter((f) => f.zone === 'main').map((f) => f.field),
    side: shown.filter((f) => f.zone !== 'main').map((f) => f.field),
    unplaced: values
      .filter((v) => !placed.has(customFieldId(v.id)))
      .sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0)),
  };
}

/** The fields a create layout requires. */
export function requiredFields(layout: readonly LayoutField[]): string[] {
  return layout.filter((f) => f.required && !f.hidden).map((f) => f.field);
}
