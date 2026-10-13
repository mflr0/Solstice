import { mount } from 'svelte';

import '@fontsource-variable/source-serif-4';
import '@fontsource/ibm-plex-sans/400.css';
import '@fontsource/ibm-plex-sans/500.css';
import '@fontsource/ibm-plex-sans/600.css';
import '@fontsource/ibm-plex-mono/400.css';
import '@fontsource/ibm-plex-mono/500.css';

import './app.css';
import App from './ui/App.svelte';

const target = document.getElementById('app');
if (!target) throw new Error('#app is missing from the page');

export default mount(App, { target });
