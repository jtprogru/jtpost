// jtpost service worker — minimal offline-tolerant cache.
// Стратегия:
//   - /ui/static/* → cache-first (immutable assets за версией CACHE_VERSION)
//   - /ui/events  → network only (SSE нельзя кешировать)
//   - остальное   → network-first с fallback на кеш (offline page)
const CACHE_VERSION = 'jtpost-v1';
const STATIC_ASSETS = [
    '/ui/static/app.css',
    '/ui/static/htmx.min.js',
    '/ui/static/upload.js',
    '/ui/static/sse.js',
    '/ui/static/icon-192.png',
    '/ui/static/icon-512.png',
];

self.addEventListener('install', (e) => {
    e.waitUntil(
        caches.open(CACHE_VERSION).then((cache) => cache.addAll(STATIC_ASSETS)).catch(() => {}),
    );
    self.skipWaiting();
});

self.addEventListener('activate', (e) => {
    e.waitUntil(
        caches.keys().then((keys) => Promise.all(
            keys.filter((k) => k !== CACHE_VERSION).map((k) => caches.delete(k)),
        )),
    );
    self.clients.claim();
});

self.addEventListener('fetch', (e) => {
    const req = e.request;
    if (req.method !== 'GET') return;
    const url = new URL(req.url);
    if (url.pathname === '/ui/events' || url.pathname.startsWith('/ui/uploads/')) {
        return; // network only
    }
    if (url.pathname.startsWith('/ui/static/')) {
        e.respondWith(
            caches.match(req).then((hit) => hit || fetch(req).then((res) => {
                if (res.ok) {
                    const copy = res.clone();
                    caches.open(CACHE_VERSION).then((c) => c.put(req, copy));
                }
                return res;
            })),
        );
        return;
    }
    if (url.pathname.startsWith('/ui/')) {
        e.respondWith(
            fetch(req).then((res) => {
                if (res.ok) {
                    const copy = res.clone();
                    caches.open(CACHE_VERSION).then((c) => c.put(req, copy));
                }
                return res;
            }).catch(() => caches.match(req)),
        );
    }
});
