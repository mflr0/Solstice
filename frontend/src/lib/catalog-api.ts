/**
 * The few calls of the catalogues' administration that src/lib/admin-api.ts
 * does not type: statuses created without a project (global
 * administrators), their order, and the custom field types offered.
 */

import type { FieldType, StatusRow } from '../core/admin-types.js';
import type { StatusCategory, StatusRef } from '../core/types.js';
import { request } from './api.js';

export const catalogApi = {
  /** Answers 201 with a new status, or 200 with the one of the same name. */
  createStatus: (s: { name: string; category: StatusCategory }) =>
    request<StatusRef>('POST', '/api/statuses', s),
  orderStatuses: (ids: string[]) =>
    request<StatusRow[]>('POST', '/api/admin/statuses/order', { ids }),
  fieldTypes: () =>
    request<{ type: FieldType; typeKey: string; options: boolean }[]>(
      'GET',
      '/api/admin/fields/types',
    ),
};
