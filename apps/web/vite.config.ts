import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  // MapLibre ships its tile-parsing Web Worker as a separate ESM entry
  // that Vite's dep pre-bundler doesn't emit, so it 404s at runtime.
  // Excluding it from pre-bundling serves it as-is instead.
  optimizeDeps: { exclude: ['maplibre-gl'] },
})
