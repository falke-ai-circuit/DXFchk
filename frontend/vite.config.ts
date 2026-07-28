import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
  ],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8643',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: '../internal/api/frontend_dist',
    assetsDir: 'assets',
    emptyOutDir: true,
  },
});