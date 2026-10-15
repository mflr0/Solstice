import { mount } from 'svelte';

import '@fontsource-variable/source-serif-4';
import '@fontsource/ibm-plex-sans/400.css';
import '@fontsource/ibm-plex-sans/500.css';
import '@fontsource/ibm-plex-sans/600.css';
import '@fontsource/ibm-plex-mono/400.css';
import '@fontsource/ibm-plex-mono/500.css';

import './app.css';
import { initLocale } from './i18n/index.js';
import App from './ui/App.svelte';

const target = document.getElementById('app');
if (!target) throw new Error('#app is missing from the page');

if (location.pathname === '/setup' || location.pathname.startsWith('/setup/')) {
  // The first-run wizard: a page of its own, with its own texts.
  void import('./ui/setup/boot.js').then((setup) => setup.startSetup(target));
} else {
  // The language first: its catalogue (French ships with the page) loads
  // before anything is drawn, so no string ever shows in the wrong language.
  void initLocale().then(() => mount(App, { target }));
}
