import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// Base path for the built site. Defaults to '/' for local dev.
// In CI (GitHub Pages) we set VITE_BASE=/Gthulhu/ so all asset URLs and
// the MSW service worker register correctly under the project subpath.
const base = process.env.VITE_BASE || '/';

export default defineConfig({
  plugins: [react()],
  base,
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '/health': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '/version': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets'
  }
});
