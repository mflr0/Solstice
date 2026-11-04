import type { ThemeManifest } from '../../src/core/appearance.js';

const swatch = { bar: '#111111', paper: '#eeeeee', ink: '#000000', accent: '#ff0000' };

function theme(id: string, name: string): ThemeManifest {
  return {
    id,
    contract: 1,
    version: '1.0.0',
    name: { en: name },
    description: { en: `${name}'s look` },
    author: 'Tests',
    schemes: ['light', 'dark'],
    preview: { light: swatch, dark: swatch },
  };
}

/** What the server would list. */
export const THEMES = [
  theme('carnet', 'Notebook'),
  theme('orbite', 'Orbit'),
  theme('luna', 'Luna'),
];
