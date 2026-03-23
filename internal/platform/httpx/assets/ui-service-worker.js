const CACHE_NAME = 'orbyte-ui-shell-v2';
const PRECACHE_URLS = [
  '/ui',
  '/ui/manifest.webmanifest',
  '/ui/assets/platform.css?v={{PLATFORM_ASSET_VERSION}}',
  '/ui/assets/ui-shell-inline.css?v={{PLATFORM_ASSET_VERSION}}',
  '/ui/assets/ui-shell.js?v={{PLATFORM_ASSET_VERSION}}'
];

self.addEventListener('install', (event) => {
  event.waitUntil(caches.open(CACHE_NAME).then((cache) => cache.addAll(PRECACHE_URLS)).then(() => self.skipWaiting()));
});

self.addEventListener('activate', (event) => {
  event.waitUntil(caches.keys().then((keys) => Promise.all(keys.filter((key) => key !== CACHE_NAME).map((key) => caches.delete(key)))).then(() => self.clients.claim()));
});

function shouldCache(requestURL) {
  return requestURL.pathname === '/ui' ||
    requestURL.pathname === '/ui/assets/platform.css' ||
    requestURL.pathname === '/ui/assets/ui-shell-inline.css' ||
    requestURL.pathname === '/ui/assets/ui-shell.js' ||
    requestURL.pathname === '/auth/options' ||
    requestURL.pathname === '/ui/bootstrap' ||
    requestURL.pathname.indexOf('/ui/routes/resolve') === 0 ||
    requestURL.pathname.indexOf('/ui/views/') === 0 ||
    requestURL.pathname.indexOf('/ui/assets/modules/') === 0;
}

self.addEventListener('fetch', (event) => {
  if (event.request.method !== 'GET') return;
  const requestURL = new URL(event.request.url);
  if (!shouldCache(requestURL)) return;
  event.respondWith(
    fetch(event.request)
      .then((response) => {
        if (response && response.ok) {
          const copy = response.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(event.request, copy));
        }
        return response;
      })
      .catch(() => caches.match(event.request).then((cached) => cached || caches.match('/ui')))
  );
});
