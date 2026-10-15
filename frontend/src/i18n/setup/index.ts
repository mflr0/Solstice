/**
 * The setup wizard's catalogues. They are small, so all ship together: the
 * wizard switches language in place, without reloading.
 */

import type { Locale } from '../locales.js';
import { de } from './de.js';
import { en } from './en.js';
import { fr } from './fr.js';
import { hi } from './hi.js';
import { it } from './it.js';
import type { SetupMessages } from './types.js';

export type { SetupMessages } from './types.js';

export const setupCatalogues: Record<Locale, SetupMessages> = { fr, en, de, it, hi };
