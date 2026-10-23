import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';

const r = (p: string) => fileURLToPath(new URL(p, import.meta.url));

// The Go server listens here in development; Vite proxies the API to it.
const backend = process.env.SOLSTICE_BACKEND ?? 'http://localhost:8080';

export default defineConfig({
  plugins: [svelte()],
  // Component tests mount the browser build of Svelte, not its server one.
  ...(process.env.VITEST ? { resolve: { conditions: ['browser'] } } : {}),
  server: {
    proxy: {
      '/api': { target: backend },
      '/rest': { target: backend },
      '/healthz': { target: backend },
    },
  },
  build: {
    // The Go binary embeds this directory, so production is a single executable.
    outDir: r('../backend/internal/web/dist'),
    emptyOutDir: true,
  },
  test: {
    include: ['tests/**/*.test.ts'],
    environment: 'node',
  },
});
