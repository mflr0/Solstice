/**
 * The shape of every setup catalogue: French's, with strings widened to
 * string and functions keeping their parameters (as src/i18n/types.ts does
 * for the application's catalogue).
 */

import type { Widen } from '../types.js';
import type { fr } from './fr.js';

export type SetupMessages = Widen<typeof fr>;
