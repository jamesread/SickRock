import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  build: {
    rollupOptions: {
      plugins: [
      ],
    },
  },
  server: {
    port: 5173,
    allowedHosts: ['mindstorm', '0.0.0.0', "baneling.teratan.net"],
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        secure: false,
      },
    },
  },
})
