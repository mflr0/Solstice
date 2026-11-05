import type { TypeIcon } from '../core/types.js';

/**
 * One path of a pictogram. Without a part it is a shape, which a fill paints.
 * A `line` is always only stroked; a `detail` sits inside a shape and is drawn
 * in the paper colour on a filled glyph.
 */
export interface TypeIconPath {
  d: string;
  part?: 'line' | 'detail';
}

const shape = (d: string): TypeIconPath => ({ d });
const line = (d: string): TypeIconPath => ({ d, part: 'line' });
const detail = (d: string): TypeIconPath => ({ d, part: 'detail' });

/** Issue type pictograms, one per icon name the server accepts, drawn on a 16 × 16 grid with round strokes. */
export const TYPE_ICONS: Record<TypeIcon, TypeIconPath[]> = {
  bug: [
    shape('M5 7a3 3 0 0 1 6 0v2.5a3 3 0 0 1-6 0z'),
    line('M6.5 4.3 5.5 2.5'),
    line('M9.5 4.3l1-1.8'),
    line('M2.5 8.5H5'),
    line('M11 8.5h2.5'),
    line('M3 12l2-1'),
    line('M13 12l-2-1'),
  ],
  story: [
    shape(
      'M8 4.5C6.5 3.5 4.5 3.3 2.5 3.5v9c2-.2 4 0 5.5 1 1.5-1 3.5-1.2 5.5-1v-9c-2-.2-4 0-5.5 1z',
    ),
    detail('M8 4.5v9'),
  ],
  epic: [shape('M1.5 13 6 5l2.5 4.2L10.5 6l4 7z')],
  task: [shape('M2.5 2.5h11v11h-11z'), detail('M5 8.2 7 10.2l4-4.4')],
  subtask: [shape('M2.5 2.5h4.5v4h-4.5z'), line('M4.75 6.5v4.75H9'), shape('M9 9h4.5v4.5H9z')],
  bolt: [shape('M9 1.5 3.5 9H8l-1 5.5L12.5 7H8z')],
  flag: [line('M3.5 14V2.5'), shape('M3.5 3h8.5l-2 3 2 3H3.5')],
  star: [
    shape(
      'M8 2.5l1.44 4.02 4.26.13-3.37 2.61 1.2 4.09L8 10.95l-3.53 2.4 1.2-4.09-3.37-2.61 4.26-.13z',
    ),
  ],
  check: [line('M3 8.5 6.5 12 13 4.5')],
  bookmark: [shape('M4 2.5h8v11L8 10.5l-4 3z')],
  question: [
    shape('M2 8a6 6 0 1 0 12 0a6 6 0 1 0-12 0'),
    detail('M6.2 6.3a1.8 1.8 0 1 1 2.6 1.6c-.5.3-.8.7-.8 1.3v.3'),
    detail('M8 11.5h.01'),
  ],
  wrench: [
    shape(
      'M10.5 2.5a3 3 0 0 0-2.8 4.1l-4.9 4.9a1.2 1.2 0 0 0 1.7 1.7l4.9-4.9a3 3 0 0 0 4.1-2.8l-1.8 1.8-1.9-.4-.4-1.9z',
    ),
  ],
  shield: [shape('M8 1.8 13 3.8v4c0 3-2.2 5.2-5 6.4-2.8-1.2-5-3.4-5-6.4v-4z')],
  doc: [
    shape('M3.5 1.5h6l3 3v10h-9z'),
    detail('M9.5 1.5v3h3'),
    detail('M5.5 8h5'),
    detail('M5.5 11h5'),
  ],
  rocket: [
    shape('M8 1.5c2.5 1.8 3.5 4.5 3 8H5c-.5-3.5.5-6.2 3-8z'),
    shape('M5 7.5 3 10v2l2-1.5'),
    shape('M11 7.5l2 2.5v2l-2-1.5'),
    line('M8 12v2.5'),
    detail('M8 6h.01'),
  ],
  lightbulb: [
    shape('M5.8 11c0-1.3-2.3-2.4-2.3-5a4.5 4.5 0 0 1 9 0c0 2.6-2.3 3.7-2.3 5z'),
    line('M6 13h4'),
    line('M6.8 14.8h2.4'),
  ],
  gear: [
    shape('M4.5 8a3.5 3.5 0 1 0 7 0a3.5 3.5 0 1 0-7 0'),
    detail('M6.8 8a1.2 1.2 0 1 0 2.4 0a1.2 1.2 0 1 0-2.4 0'),
    line('M8 2v2.5'),
    line('M8 11.5V14'),
    line('M2 8h2.5'),
    line('M11.5 8H14'),
    line('M3.75 3.75l1.77 1.77'),
    line('M10.48 10.48l1.77 1.77'),
    line('M3.75 12.25l1.77-1.77'),
    line('M10.48 5.52l1.77-1.77'),
  ],
  lock: [shape('M3.5 7.5h9v6h-9z'), line('M5.5 7.5V5a2.5 2.5 0 0 1 5 0v2.5')],
};

/** The glyph of an icon name, or null for none or a name this build doesn't draw. */
export function typeIcon(name: string | undefined): TypeIconPath[] | null {
  return name && Object.hasOwn(TYPE_ICONS, name) ? TYPE_ICONS[name as TypeIcon] : null;
}
