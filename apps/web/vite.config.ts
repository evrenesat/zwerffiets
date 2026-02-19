import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';
import path from 'node:path';

const BUILD_ID = process.env.BUILD_ID ?? 'dev';
const DEV_API_PROXY_TARGET = process.env.DEV_API_PROXY_TARGET ?? 'http://127.0.0.1:8080';

export default defineConfig({
  plugins: [sveltekit()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    strictPort: true,
    proxy: {
      '/api/v1': {
        target: DEV_API_PROXY_TARGET,
        changeOrigin: true
      },
      '/bikeadmin': {
        target: DEV_API_PROXY_TARGET,
        changeOrigin: true
      }
    }
  },
  preview: {
    host: '0.0.0.0'
  },
  define: {
    __BUILD_ID__: JSON.stringify(BUILD_ID)
  },
  resolve: {
    alias: {
      $server: path.resolve('./src/lib/server'),
      $lib: path.resolve('./src/lib')
    }
  },
  test: {
    include: ['src/**/*.{test,spec}.ts'],
    environment: 'node',
    coverage: {
      reporter: ['text', 'html'],
      include: ['src/lib/server/**/*.ts']
    }
  }
});
