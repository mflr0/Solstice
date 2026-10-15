# Interface languages

French (`fr.ts`) is the source text. Every other language is one file of the
same shape: `en.ts`, `de.ts`, `it.ts`, `hi.ts`.

## Files

- `fr.ts` — the French catalogue (`export const fr = { … } as const`) and the
  live binding `t` every component reads.
- `types.ts` — `Messages`, the shape of every catalogue: French's, with strings
  widened to `string` and functions keeping their parameters.
- `xx.ts` — `export const xx: Messages = { … }`. TypeScript reports any key a
  catalogue lacks or adds (`pnpm lint`).
- `format.ts` — dates, numbers, plurals, lists and relative times, built on
  `Intl` for the language in use. Use these helpers instead of writing month
  names or plural rules by hand: `plural(n, one, other)`, `formatNumber`,
  `formatList`, `timeAgo`, `longDay`, `shortDay`, `dayMonth`, `dateTime`.
- `locales.ts` — the supported languages, their own names, how one is chosen.
- `index.ts` — loads a catalogue at boot (one language per visit).

## Translating

- Translate meaning, not words: natural, short interface text, sentence case.
- Keep every key, every function and its parameters; only the strings change.
  Keep the values a sentence needs (`${n}`, `${name}`).
- Plurals go through the `plural` helper (or `Intl.PluralRules` when a language
  needs more forms).
- Don't translate user data (status names, project names…), keys like `DEMO-12`,
  `JQL`, JQL field names in examples (`assignee = currentUser()`), or the
  product name Solstice.
- Mentions of the source system in migration texts stay as in French.
- Typography: German „…“, Italian «…», English and Hindi “…”; Hindi ends
  sentences with « । ». Non-breaking spaces before punctuation are French only.
- Tone: German uses « Sie », Italian uses « tu », Hindi uses « आप ».

## Glossary

| French | English | Deutsch | Italiano | हिन्दी |
| --- | --- | --- | --- | --- |
| ticket | issue | Vorgang | ticket | टिकट |
| projet | project | Projekt | progetto | प्रोजेक्ट |
| tableau (board) | board | Board | board | बोर्ड |
| sprint | sprint | Sprint | sprint | स्प्रिंट |
| épopée | epic | Epic | epic | एपिक |
| workflow | workflow | Workflow | workflow | वर्कफ़्लो |
| statut | status | Status | stato | स्टेटस |
| transition | transition | Übergang | transizione | ट्रांज़िशन |
| sous-tâche | sub-task | Unteraufgabe | sotto-task | सब-टास्क |
| responsable | assignee | Bearbeiter | assegnatario | असाइनी |
| rapporteur | reporter | Melder | segnalatore | रिपोर्टर |
| observateur | watcher | Beobachter | osservatore | वॉचर |
| priorité | priority | Priorität | priorità | प्राथमिकता |
| résolution | resolution | Lösung | risoluzione | रिज़ॉल्यूशन |
| filtre | filter | Filter | filtro | फ़िल्टर |
| filtre rapide | quick filter | Schnellfilter | filtro rapido | क्विक फ़िल्टर |
| couloir | swimlane | Swimlane | corsia | स्विमलेन |
| étiquette | label | Label | etichetta | लेबल |
| composant | component | Komponente | componente | कंपोनेंट |
| version corrigée | fix version | Lösungsversion | versione di correzione | फ़िक्स वर्ज़न |
| commentaire | comment | Kommentar | commento | टिप्पणी |
| pièce jointe | attachment | Anhang | allegato | अटैचमेंट |
| temps passé | time spent | erfasste Zeit | tempo impiegato | लगा समय |
| fiche (record card) | details | Details | dettagli | विवरण |
| thème | theme | Design | tema | थीम |
| réglages | settings | Einstellungen | impostazioni | सेटिंग्स |
| droits | permissions | Berechtigungen | permessi | अनुमतियाँ |
| Mon travail | My work | Meine Arbeit | Il mio lavoro | मेरा काम |

Skin names: Carnet → Notebook / Notizbuch / Taccuino / नोटबुक; Orbite → Orbit /
Orbit / Orbita / ऑर्बिट; Crayonné → Sketch / Skizze / Schizzo / स्केच; Luna,
Rétro 98 and Terminal stay; Classique → Classic / Klassisch / Classico /
क्लासिक; Plan → Blueprint / Blaupause / Cianografia / ब्लूप्रिंट.

## Checking

`pnpm lint` (types, every key present) and `pnpm test`. Then look at the pages
in the language: German is long — layouts must wrap or truncate, never break.
