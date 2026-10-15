/**
 * The shape every catalogue has: French's (src/i18n/fr.ts), with each string
 * literal widened to string and each function keeping its parameters. A
 * locale file is `export const xx: Messages = { … }`; TypeScript then
 * reports any key it lacks or adds.
 */

import type { fr } from './fr.js';

export type Widen<T> = T extends string
  ? string
  : T extends (...args: infer A) => infer R
    ? (...args: A) => Widen<R>
    : T extends readonly (infer U)[]
      ? readonly Widen<U>[]
      : T extends object
        ? { readonly [K in keyof T]: Widen<T[K]> }
        : T;

export type Messages = Widen<typeof fr>;
