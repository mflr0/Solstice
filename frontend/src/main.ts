import { mount } from 'svelte';

import './app.css';
import './themes/carnet.css';
import { initLocale } from './i18n/index.js';
import App from './ui/App.svelte';

const target = document.getElementById('app');
if (!target) throw new Error('#app is missing from the page');

if (location.pathname === '/setup' || location.pathname.startsWith('/setup/')) {
  // The first-run wizard: a page of its own, with its own texts.
  void import('./ui/setup/boot.js').then((setup) => setup.startSetup(target));
} else {
  // The language first: its catalogue (English ships with the page) loads
  // before anything is drawn, so no string ever shows in the wrong language.
  void initLocale().then(() => mount(App, { target }));
}
