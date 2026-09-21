// Where the UI gets its data: the local server, or - in a static export - the JSON
// embedded in the page by web.WriteStatic.
//
// Normally the server swaps the token in the address for a cookie as the page loads,
// and nothing here has to think about it again. Inside an editor's built-in browser
// (depphunter --embed) it cannot: the page is framed by another origin, a cookie set
// here is a third-party cookie there, and browsers do not send those back. So the
// token stays in the address and every call carries it - in a header where one can
// be set, and in the query string for the event stream, which cannot set headers.
//
// In the ordinary case there is no token in the address to find, because the server
// took it out, and all of this costs nothing.

const embedded = document.getElementById('depphunter-data');
export const STATIC = embedded ? JSON.parse(embedded.textContent) : null;

const token = new URLSearchParams(location.search).get('token') || '';

/** The headers every call carries: the token, when the page was given one. */
export const auth = (more = {}) => (token ? { ...more, 'X-Depphunter-Token': token } : more);

/** The same token on a URL, for the requests that cannot carry a header. */
export const authed = url => (token ? url + (url.includes('?') ? '&' : '?') + `token=${encodeURIComponent(token)}` : url);

export async function fetchGraph() {
  if (STATIC) return { graph: STATIC.graph, version: 1 };
  const res = await fetch('api/graph', { headers: auth() });
  if (!res.ok) throw new Error(`api/graph: ${res.status} ${await res.text()}`);
  return { graph: await res.json(), version: +res.headers.get('X-Graph-Version') || 0 };
}

export async function fetchConfig() {
  if (STATIC) return STATIC.config;
  const res = await fetch('api/config', { headers: auth() });
  if (!res.ok) throw new Error(`api/config: ${res.status} ${await res.text()}`);
  return res.json();
}

export async function fetchSource(path) {
  if (STATIC) {
    const src = STATIC.sources?.[path];
    if (src == null) throw new Error('source not included in this export (too large or binary)');
    return src;
  }
  const res = await fetch(`api/file?path=${encodeURIComponent(path)}`, { headers: auth() });
  const text = await res.text();
  if (!res.ok) throw new Error(`${res.status} ${text.trim()}`);
  return text;
}

export async function saveSettings(ui) {
  const res = await fetch('api/settings', {
    method: 'POST',
    headers: auth({ 'Content-Type': 'application/json', 'X-Depphunter-Request': '1' }),
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
  const res = await fetch(`api/${name}`, { headers: auth() });
  if (res.status === 202) return 'pending';
  if (res.status === 204) return null;
  if (!res.ok) throw new Error(`api/${name}: ${res.status} ${await res.text()}`);
  return res.json();
}
