// Talking to a running depphunter from the extension host. The map's page and this
// extension are two views of one server (internal/server).
//
// node:http rather than fetch, because of the event stream: a request whose body is
// read line by line is the shortest way to follow Server-Sent Events without a client
// library. The server listens on this machine's loopback, which is where the
// extension host is, so the address it printed is the address to use - not the one
// asExternalUri rewrites for a browser elsewhere. That address is also the only place
// the session token appears, and every call carries it in a header.

import * as http from 'node:http';
import * as vscode from 'vscode';

import { Graph } from './graph';

// The graph document's own types are declared once, in Go, and generated from there
// (internal/graph/types_test.go): a hand copy of a struct is a field added on one
// side and quietly not read on the other. They are re-exported so that everything
// here still has one place to import from.
export type { Graph, GraphNode, GraphEdge } from './graph';


/** One caught finding, as the map's backpack records it. */
export interface PackItem {
  id: string;
  severity: string;
  title: string;
  where: string;
  line?: number;
  nodeId?: string;
  caughtAt?: number;
  fixed?: boolean;
}

export interface Session {
  selected: string;
  backpack: PackItem[];
}

/** What the server announces on its event stream, by name. */
export type ServerEvent =
  | { name: 'graph'; data: { version: number; changed?: string[] } }
  | { name: 'selection'; data: { id: string; origin: string } }
  | { name: 'backpack'; data: { count: number; origin: string } }
  // The greeting every connection opens with. `resumed` says the stream carried on
  // from the last event this client saw, so nothing was announced while it was away;
  // without it a reconnection is indistinguishable from a first connection, and the
  // panel carries on listening without learning what it slept through.
  | { name: 'hello'; data: { version: number; etag: string; seq: number; resumed: boolean } };

/** How long a call waits for the server's answer to start. */
const REQUEST_TIMEOUT = 30_000;

export class Api {
  private readonly base: string;
  private readonly token: string;
  /** The entity tag of the graph last read, so an unchanged one is not sent again. */
  private graphTag = '';
  /** The id of the last announcement seen, handed back when the stream reconnects. */
  private lastEvent = '';
  /** This client, so our own changes coming back are not applied a second time. */
  readonly origin = `vscode-${Math.random().toString(36).slice(2, 10)}`;

  /** address is what the server printed: its URL with the session token in the query. */
  constructor(address: string) {
    const url = new URL(address);
    this.token = url.searchParams.get('token') ?? '';
    url.search = '';
    this.base = url.toString().replace(/\/$/, '');
  }

  /**
   * The graph, or null when the server says this client already has it. `force` asks
   * for it whatever the server thinks, which is what the panel's Refresh means.
   *
   * The document is the largest thing the panel reads and rarely differs between
   * asks - a reconnect, a server restarted under an open window. Its fingerprint is
   * of the nodes and edges rather than of when they were read, so those answer 304.
   */
  async graph(force = false): Promise<Graph | null> {
    const res = await this.fetch('GET', '/api/graph', undefined,
      !force && this.graphTag ? { 'If-None-Match': this.graphTag } : {});
    if (res.status === 304) return null;
    this.graphTag = res.etag;
    return JSON.parse(res.body.toString('utf8')) as Graph;
  }

  session(): Promise<Session> {
    return this.json<Session>('GET', '/api/session');
  }

  /** Says what the side panel picked; the map follows. */
  select(id: string): Promise<void> {
    return this.send('POST', '/api/selection', { id, origin: this.origin });
  }

  /** Hands the backpack back with something taken out of it. */
  setBackpack(items: PackItem[]): Promise<void> {
    return this.send('PUT', '/api/backpack', { items, origin: this.origin });
  }

  /** An export, as bytes, for whatever the user chose to save it as. */
  download(path: string): Promise<Buffer> {
    return this.request('GET', path);
  }

  /**
   * Follows the server's event stream until the token is cancelled or the server
   * stops. The stream is the only way the panel hears about a selection made on the
   * map, so it reconnects - a --watch re-analysis does not drop it, but a restart
   * of the editor's own network stack can.
   */
  watch(onEvent: (event: ServerEvent) => void): vscode.Disposable {
    let stopped = false;
    let request: http.ClientRequest | undefined;
    let retry: NodeJS.Timeout | undefined;

    const connect = () => {
      if (stopped) return;
      // A connection ends with 'error' before a response, or 'close' after one, and
      // may report both: only the first of them schedules the next attempt.
      let over = false;
      const again = () => {
        if (over || stopped) return;
        over = true;
        retry = setTimeout(connect, wait);
        wait = Math.min(wait * 2, 30_000);
      };
      // Last-Event-ID is how the greeting knows whether this is a reconnection, and
      // therefore whether anything was announced while the connection was down.
      const resume = this.lastEvent ? { 'Last-Event-ID': this.lastEvent } : {};
      request = this.open('GET', '/api/events', undefined, res => {
        if (res.statusCode !== 200) {
          res.resume();
          again();
          return;
        }
        // Reset on connecting, so a later drop is retried promptly.
        wait = 1000;
        // An event is a few lines and a blank one; only whole events are parsed, so
        // one split across two reads is not half-read.
        let pending = '';
        res.setEncoding('utf8');
        res.on('data', (chunk: string) => {
          pending += chunk;
          let cut = pending.indexOf('\n\n');
          for (; cut >= 0; cut = pending.indexOf('\n\n')) {
            const block = pending.slice(0, cut);
            pending = pending.slice(cut + 2);
            const parsed = parseEvent(block);
            if (!parsed) continue;
            // Remembered before it is handed on, so a reconnect says where this
            // client got to even if handling the event throws.
            if (parsed.id) this.lastEvent = parsed.id;
            onEvent(parsed.event);
          }
          if (pending.length > 64 << 10) pending = ''; // not an event; nothing to wait for
        });
        // 'close' rather than 'end': a connection reset mid-stream never ends.
        res.on('close', again);
      }, resume);
      request.on('error', again);
      request.end();
    };

    // The server may simply have been stopped, and an extension that reconnected
    // forever would be a request a second forever. Slowing down is enough: the view
    // is refreshed from scratch whenever a server is started again.
    let wait = 1000;

    connect();
    return new vscode.Disposable(() => {
      stopped = true;
      clearTimeout(retry);
      request?.destroy();
    });
  }

  private async json<T>(method: string, path: string): Promise<T> {
    return JSON.parse((await this.request(method, path)).toString('utf8')) as T;
  }

  private async send(method: string, path: string, body: unknown): Promise<void> {
    await this.request(method, path, JSON.stringify(body));
  }

  /** The body of a successful call; anything else is an error. */
  private async request(method: string, path: string, body?: string): Promise<Buffer> {
    return (await this.fetch(method, path, body)).body;
  }

  /**
   * One call, with the status and the entity tag as well as the body. 304 is an
   * answer rather than a failure - it is what a conditional request asks for - so it
   * comes back to the caller instead of being thrown.
   */
  private fetch(method: string, path: string, body?: string, extra: Record<string, string> = {}):
  Promise<{ status: number; body: Buffer; etag: string }> {
    return new Promise((resolve, reject) => {
      const req = this.open(method, path, body, res => {
        const chunks: Buffer[] = [];
        res.on('data', (c: Buffer) => chunks.push(c));
        res.on('end', () => {
          const data = Buffer.concat(chunks);
          const status = res.statusCode ?? 0;
          if ((status >= 200 && status < 300) || status === 304) {
            resolve({ status, body: data, etag: res.headers.etag ?? '' });
          } else {
            reject(new Error(`${method} ${path}: ${status} ${data.toString('utf8').trim()}`));
          }
        });
      }, extra);
      req.on('error', reject);
      // A server that accepts the connection and never answers would hold the caller,
      // and the map waiting on it, indefinitely. The event stream has no timeout: it
      // is quiet by design.
      req.setTimeout(REQUEST_TIMEOUT, () => req.destroy(new Error(`${method} ${path}: no answer in ${REQUEST_TIMEOUT / 1000}s`)));
      if (body !== undefined) req.write(body);
      req.end();
    });
  }

  private open(method: string, path: string, body: string | undefined,
    onResponse: (res: http.IncomingMessage) => void,
    extra: Record<string, string> = {}): http.ClientRequest {
    const url = new URL(this.base + path);
    const headers: Record<string, string> = {
      'Accept': 'application/json',
      // The server refuses a write without it: a cross-site page cannot set a header
      // without a preflight, and it grants none.
      'X-Depphunter-Request': '1',
      ...extra,
    };
    if (this.token) headers['X-Depphunter-Token'] = this.token;
    if (body !== undefined) {
      headers['Content-Type'] = 'application/json';
      headers['Content-Length'] = String(Buffer.byteLength(body));
    }
    return http.request({
      protocol: url.protocol,
      hostname: url.hostname,
      port: url.port,
      path: url.pathname + url.search,
      method,
      headers,
    }, onResponse);
  }
}

/**
 * Reads one block off the stream: its `id:`, which is what a reconnection hands back,
 * and the `event:` / `data:` it carries. undefined for a comment or a heartbeat.
 */
function parseEvent(block: string): { id: string; event: ServerEvent } | undefined {
  let name = 'message';
  let id = '';
  const data: string[] = [];
  for (const line of block.split('\n')) {
    if (line.startsWith(':')) continue; // a heartbeat
    const [field, ...rest] = line.split(':');
    const value = rest.join(':').replace(/^ /, '');
    if (field === 'id') id = value;
    if (field === 'event') name = value;
    if (field === 'data') data.push(value);
  }
  if (!data.length) return undefined;
  try {
    return { id, event: { name, data: JSON.parse(data.join('\n')) } as ServerEvent };
  } catch {
    return undefined;
  }
}
