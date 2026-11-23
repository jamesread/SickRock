import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import basicSsl from '@vitejs/plugin-basic-ssl'
import hugeicons from './vite-plugin-hugeicons.js'

const backendPort = process.env.PORT || '8080'
const backendTarget = `http://localhost:${backendPort}`

export default defineConfig({
  plugins: [
    vue(),
    basicSsl(),
    hugeicons(),
  ],
  // picocrank ships source (.vue and .js). Pre-bundling it would give a .js import (e.g. its
  // notification popups composable) a second copy of module state, apart from the copy its .vue
  // components import, so serve it as is in dev, as the production build does.
  optimizeDeps: {
    exclude: ['picocrank'],
  },
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
        target: backendTarget,
        changeOrigin: true,
        secure: false,
      },
      // Built by the SickRock server (named after the instance); see service/internal/pwa.
      '/manifest.json': {
        target: backendTarget,
        changeOrigin: true,
        secure: false,
      },
      '/metrics': {
        target: backendTarget,
        changeOrigin: true,
        secure: false,
      },
    },
  },
})
