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

// Implements: REQ-EXP-008
const embedded = document.getElementById('depphunter-data');
export const STATIC = embedded ? JSON.parse(embedded.textContent) : null;

const token = new URLSearchParams(location.search).get('token') || '';

/** The headers every call carries: the token, when the page was given one. */
const auth = (more = {}) => (token ? { ...more, 'X-Depphunter-Token': token } : more);

/** The same token on a URL, for the requests that cannot carry a header. */
export const authed = url => (token ? url + (url.includes('?') ? '&' : '?') + `token=${encodeURIComponent(token)}` : url);

// The entity tag of the graph this page last read, so an unchanged one answers 304.
let graphTag = '';

/**
 * The graph, or null when the server says the page already has it.
 *
 * The document is the largest thing here - twenty megabytes of JSON on a repository
 * of a hundred thousand nodes - and most of the times it is asked for, nothing about
 * it has changed: a reconnect, a reload, a server restarted under a page that was
 * left open. The fingerprint is of the nodes and the edges rather than of when they
 * were read, so all of those answer 304 and cost nothing to parse.
 *
 * Implements: REQ-SRV-017
 */
export async function fetchGraph() {
  if (STATIC) return { graph: STATIC.graph, version: 1 };
  const response = await fetch('api/graph', { headers: auth(graphTag ? { 'If-None-Match': graphTag } : {}) });
  if (response.status === 304) return { graph: null, version: +response.headers.get('X-Graph-Version') || 0 };
  if (!response.ok) throw new Error(`api/graph: ${response.status} ${await response.text()}`);
  graphTag = response.headers.get('ETag') || '';
  return { graph: await response.json(), version: +response.headers.get('X-Graph-Version') || 0 };
}

export async function fetchConfig() {
  if (STATIC) return STATIC.config;
  const response = await fetch('api/config', { headers: auth() });
  if (!response.ok) throw new Error(`api/config: ${response.status} ${await response.text()}`);
  return response.json();
}

/** A file that has no text to show, and what the server says it is instead. */
export class BinaryFile extends Error {
  constructor(type) {
    super('binary file');
    this.type = type || 'application/octet-stream';
  }
}

export async function fetchSource(path) {
  if (STATIC) {
    const source = STATIC.sources?.[path];
    if (source == null) throw new Error('source not included in this export (too large or binary)');
    return source;
  }
  const response = await fetch(`api/file?path=${encodeURIComponent(path)}`, { headers: auth() });
  if (response.status === 415) throw new BinaryFile(response.headers.get('X-Depphunter-Binary'));
  const text = await response.text();
  if (!response.ok) throw new Error(`${response.status} ${text.trim()}`);
  return text;
}

/**
 * Where a file's own bytes are served, for an <img>, a <video> or an <audio> to load
 * by itself - which is why the token rides on the URL rather than in a header - or
 * null in a static export, which carries no bytes.
 *
 * Implements: REQ-MAP-062
 */
export const fileURL = path => (STATIC ? null : authed(`api/file?as=raw&path=${encodeURIComponent(path)}`));

/** The first `limit` bytes of a file, asked for as a range so a large one is not read whole. */
export async function fetchBytes(path, limit) {
  if (STATIC) throw new Error('not included in this export');
  const response = await fetch(`api/file?as=raw&path=${encodeURIComponent(path)}`,
    { headers: auth({ Range: `bytes=0-${limit - 1}` }) });
  if (!response.ok) throw new Error(`${response.status} ${(await response.text()).trim()}`);
  return new Uint8Array(await response.arrayBuffer()).subarray(0, limit);
}

/** A change sent to the server: JSON, marked as the page's own request. */
export const send = (url, method, body) => fetch(url, {
  method,
  headers: auth({ 'Content-Type': 'application/json', 'X-Depphunter-Request': '1' }),
  body: JSON.stringify(body),
});

// Implements: REQ-CFG-012
export async function saveSettings(ui) {
  const response = await send('api/settings', 'POST', ui);
  if (!response.ok) throw new Error((await response.text()).trim());
}

/**
 * A dataset the server computes after startup ("history", "references"): its value,
 * 'pending' while it is computed, or null when there is none.
 *
 * Implements: REQ-HIST-007, REQ-LSP-005, REQ-FND-022
 */
export async function fetchLazy(name) {
  if (STATIC) return STATIC[name] || null;
  const response = await fetch(`api/${name}`, { headers: auth() });
  if (response.status === 202) return 'pending';
  if (response.status === 204) return null;
  if (!response.ok) throw new Error(`api/${name}: ${response.status} ${await response.text()}`);
  return response.json();
}

// ---------------------------------------------------------------- shared session
//
// What is selected and what has been caught are shared with whatever else is looking
// at the same server - the editor's side panel, another tab - through /api/session.
// Everything here is best-effort: a map whose selection did not reach the panel is
// still a map, so a failed call is dropped rather than shown.

/**
 * Who this client is. Every change carries it, and the announcement carries it back,
 * so a client can tell its own change returning from somebody else's - without which
 * two of them watching each other would never settle.
 *
 * Implements: REQ-SRV-012
 */
export const CLIENT = `page-${Math.random().toString(36).slice(2, 10)}`;

/** The shared state as it stands, or null where there is no server to ask. */
export async function fetchSession() {
  if (STATIC) return null;
  try {
    const response = await fetch('api/session', { headers: auth() });
    return response.ok ? await response.json() : null;
  } catch {
    return null;
  }
}

const write = (url, method, body) => {
  if (STATIC) return Promise.resolve();
  return send(url, method, { ...body, origin: CLIENT }).catch(() => {});
};

/** Says what is selected now; '' for nothing. */
export const pushSelection = id => write('api/selection', 'POST', { id: id || '' });

/** Hands up the backpack as it now stands. */
export const pushBackpack = items => write('api/backpack', 'PUT', { items });
