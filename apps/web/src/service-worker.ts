/// <reference lib="webworker" />
import { build, files, version } from '$service-worker';

const CACHE_NAME = `zwerffiets-${version}`;
const ASSETS = [...build, ...files];
const STATIC_ASSET_PATHS = new Set(ASSETS);

const shouldCacheStaticAsset = (request: Request): boolean => {
  const url = new URL(request.url);

  if (url.origin !== self.location.origin) {
    return false;
  }

  if (STATIC_ASSET_PATHS.has(url.pathname)) {
    return true;
  }

  return url.pathname === '/manifest.webmanifest';
};

const fetchAndCache = async (request: Request): Promise<Response> => {
  const response = await fetch(request);

  if (response.ok) {
    const cache = await caches.open(CACHE_NAME);
    await cache.put(request, response.clone());
  }

  return response;
};

self.addEventListener('install', (event) => {
  event.waitUntil(
    (async () => {
      const cache = await caches.open(CACHE_NAME);
      await cache.addAll(ASSETS);
      await self.skipWaiting();
    })()
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    (async () => {
      const keys = await caches.keys();
      await Promise.all(keys.filter((key) => key !== CACHE_NAME).map((key) => caches.delete(key)));
      await self.clients.claim();
    })()
  );
});

self.addEventListener('message', (event) => {
  if ((event.data as { type?: string } | null)?.type === 'SKIP_WAITING') {
    void self.skipWaiting();
  }
});

self.addEventListener('fetch', (event) => {
  if (event.request.method !== 'GET') {
    return;
  }

  if (event.request.mode === 'navigate') {
    event.respondWith(
      fetch(event.request).catch(async () => {
        const fallback = await caches.match('/');
        return fallback ?? new Response('Offline', { status: 503 });
      })
    );
    return;
  }

  if (!shouldCacheStaticAsset(event.request)) {
    return;
  }

  event.respondWith(
    (async () => {
      const cached = await caches.match(event.request);
      if (cached) {
        return cached;
      }

      return await fetchAndCache(event.request);
    })()
  );
});
