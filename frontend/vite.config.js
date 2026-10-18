import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import basicSsl from '@vitejs/plugin-basic-ssl'
import hugeicons from './vite-plugin-hugeicons.js'

export default defineConfig({
  plugins: [
    vue(),
    basicSsl(),
    hugeicons(),
  ],
  build: {
    rollupOptions: {
      plugins: [
      ],
    },
  },
  server: {
    port: 5173,
    https: true,
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
