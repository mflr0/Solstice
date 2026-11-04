import { ADMIN_ICONS } from '../admin/admin-icons.js';

/** A pictogram per board view, drawn on the same grid as the administration's. */
export const BOARD_VIEW_ICONS = {
  board: ['M2.5 3h11v10h-11z', 'M6.17 3v10', 'M9.83 3v10'],
  backlog: ['M6 4h7.5', 'M6 8h7.5', 'M6 12h7.5', 'M2.5 4h.01', 'M2.5 8h.01', 'M2.5 12h.01'],
  reports: ['M2.5 2.5v11h11', 'M6 10.5V8', 'M9 10.5V4.5', 'M12 10.5V7'],
  settings: ADMIN_ICONS.settings,
  share: [
    'M10.25 3.5a1.75 1.75 0 1 0 3.5 0a1.75 1.75 0 1 0-3.5 0',
    'M2.25 8a1.75 1.75 0 1 0 3.5 0a1.75 1.75 0 1 0-3.5 0',
    'M10.25 12.5a1.75 1.75 0 1 0 3.5 0a1.75 1.75 0 1 0-3.5 0',
    'M5.5 7.1l5-2.7',
    'M5.5 8.9l5 2.7',
  ],
};
