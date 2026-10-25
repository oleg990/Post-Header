// Кэш для быстрого открытия и работы без сети. Страница и скрипты берутся
// из сети, а из кэша — только если сети нет, чтобы новая версия приходила сразу.
// Тяжёлый движок (ph.wasm) отдаётся из кэша, а свежая копия подтягивается в фоне.
const CACHE = 'ph-web-2';
self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', e => e.waitUntil(
  caches.keys().then(ks => Promise.all(ks.filter(k => k !== CACHE).map(k => caches.delete(k)))).then(() => self.clients.claim())
));
self.addEventListener('fetch', e => {
  const r = e.request;
  if (r.method !== 'GET' || !r.url.startsWith(self.location.origin)) return;
  const heavy = r.url.endsWith('.wasm');
  e.respondWith(caches.open(CACHE).then(async c => {
    const hit = await c.match(r);
    const net = fetch(r, heavy ? undefined : { cache: 'no-cache' }).then(res => { if (res.ok) c.put(r, res.clone()); return res; });
    if (heavy && hit) { net.catch(() => {}); return hit; }
    return net.catch(() => hit || Response.error());
  }));
});
