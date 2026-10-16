/**
 * The shape of every setup catalogue: English's, with strings widened to
 * string and functions keeping their parameters (as src/i18n/types.ts does
 * for the application's catalogue).
 */

import type { Widen } from '../types.js';
import type { en } from './en.js';

export type SetupMessages = Widen<typeof en>;
