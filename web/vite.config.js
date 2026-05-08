import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        // Disable code splitting: bundle everything into a single JS file.
        // This prevents "Failed to fetch dynamically imported module" errors
        // when Go's embed.FS is out of sync with the built index.html
        // (i.e. after recompiling Go without rebuilding the frontend first).
        manualChunks: undefined,
        inlineDynamicImports: true,
      },
    },
  },
  server: {
    proxy: {
      '/api': 'http://localhost:7777'
    }
  }
})
