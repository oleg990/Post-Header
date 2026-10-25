// Рендер в отдельном потоке: страница не подвисает, пока Go-движок рисует пост.
importScripts('wasm_exec.js');
let ready = false;
const queue = [];

self.phReady = () => { ready = true; queue.splice(0).forEach(run); postMessage({ ready: true }); };

const go = new Go();
WebAssembly.instantiateStreaming(fetch('ph.wasm'), go.importObject)
  .then(r => go.run(r.instance))
  .catch(async () => {
    const buf = await (await fetch('ph.wasm')).arrayBuffer();
    go.run((await WebAssembly.instantiate(buf, go.importObject)).instance);
  })
  .catch(e => postMessage({ fatal: String(e) }));

function run(m) {
  const t = Date.now();
  const r = self.phRender(JSON.stringify(m.params), m.photo || null, m.inline || null);
  if (r.error) { postMessage({ id: m.id, error: r.error }); return; }
  const pages = Array.from(r.pages).map(u => u.slice());
  postMessage({ id: m.id, pages, size: r.size, ms: Date.now() - t }, pages.map(p => p.buffer));
}

onmessage = e => (ready ? run(e.data) : queue.push(e.data));
