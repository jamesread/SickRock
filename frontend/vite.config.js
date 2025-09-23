import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { visualizer } from 'rollup-plugin-visualizer'

export default defineConfig({
  plugins: [vue()],
  build: {
    rollupOptions: {
      plugins: [
        visualizer({
          open: true, // opens the report in your browser
          filename: 'stats.html',
          gzipSize: true,
          brotliSize: true,
        }),
      ],
    },
  },
  server: {
    port: 5173,
    allowedHosts: ['mindstorm', '0.0.0.0'],
    proxy: {
      '/api': {
        target: 'http://localhost:8081',
        changeOrigin: true,
        secure: false,
      },
    },
  },
})
