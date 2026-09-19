// Where the UI gets its data: the local server, or — in a static export — the JSON
// embedded in the page by web.WriteStatic.

const embedded = document.getElementById('depphunter-data');
export const STATIC = embedded ? JSON.parse(embedded.textContent) : null;

export async function fetchGraph() {
  if (STATIC) return { graph: STATIC.graph, version: 1 };
  const res = await fetch('api/graph');
  if (!res.ok) throw new Error(`api/graph: ${res.status} ${await res.text()}`);
  return { graph: await res.json(), version: +res.headers.get('X-Graph-Version') || 0 };
}

export async function fetchConfig() {
  if (STATIC) return STATIC.config;
  const res = await fetch('api/config');
  if (!res.ok) throw new Error(`api/config: ${res.status} ${await res.text()}`);
  return res.json();
}

export async function fetchSource(path) {
  if (STATIC) {
    const src = STATIC.sources?.[path];
    if (src == null) throw new Error('source not included in this export (too large or binary)');
    return src;
  }
  const res = await fetch(`api/file?path=${encodeURIComponent(path)}`);
  const text = await res.text();
  if (!res.ok) throw new Error(`${res.status} ${text.trim()}`);
  return text;
}

export async function saveSettings(ui) {
  const res = await fetch('api/settings', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-Depphunter-Request': '1' },
    body: JSON.stringify(ui),
  });
  if (!res.ok) throw new Error((await res.text()).trim());
}

/**
 * A dataset the server computes after startup ("history", "references"): its value,
 * 'pending' while it is computed, or null when there is none.
 */
export async function fetchLazy(name) {
  if (STATIC) return STATIC[name] || null;
  const res = await fetch(`api/${name}`);
  if (res.status === 202) return 'pending';
  if (res.status === 204) return null;
  if (!res.ok) throw new Error(`api/${name}: ${res.status} ${await res.text()}`);
  return res.json();
}
