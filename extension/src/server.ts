// Starting a depphunter server for a folder, and reading back where it is listening.
//
// Three of the arguments are not optional and are not the user's to change:
//
//   --no-open       the extension opens the map, in the tab the user asked for
//   --addr 127.0.0.1:0   the operating system picks a free port; nothing is guessed
//   --embed         without it the page refuses to be shown in a frame at all
//
// That last one is what makes this work. The built-in browser is a webview, and a
// page inside a webview is framed by an origin belonging to the editor, so depphunter
// has to be told that that origin may frame it - and, because a cookie set by the map
// would be a third-party cookie there and would not come back, it keeps the session
// token in the address instead. The address it prints is therefore the only place
// that token appears, which is why it is read from the server's own output rather
// than put together from the port.

import { ChildProcess, spawn } from 'node:child_process';
import * as vscode from 'vscode';

import { binaryFor } from './binary';
import { editorTemplate } from './launcher';

// The origins the map may be framed by.
//
// frame-ancestors is checked against every frame above the page, not just the one
// holding it, and the built-in browser is three deep: the editor's window frames the
// webview, the webview frames the extension's page, and that page frames the map. So
// naming the webview alone leaves the window above it violating the policy, and the
// browser refuses the page before it loads - an empty tab, with the reason only in
// the webview's own developer tools.
//
//   vscode-webview:          the webview, which is given an origin of its own for
//                            every session (vscode-webview://<uuid>), so there is
//                            no exact name to give
//   vscode-file:             the editor's window, which is served from
//                            vscode-file://vscode-app in the desktop editor
//   https://*.vscode-cdn.net the webview in the browser build
//
// The browser build's window is whatever page the editor is being served from, which
// cannot be known from here - one of the two reasons the browser build is not
// supported (the other is in the README: the server answers to loopback only).
// Another editor with another scheme can be added with --embed through
// depphunter.args, which are passed after these.
const FRAME_ORIGINS = ['vscode-webview:', 'vscode-file:', 'https://*.vscode-cdn.net'];

const READY = /\bserving at (\S+)/;

export interface Running {
  readonly url: string;
  readonly child: ChildProcess;
}

export class StartError extends Error {
  constructor(message: string, readonly notFound = false) {
    super(message);
  }
}

/**
 * Starts a server for root and resolves once it says where it is listening. home is
 * the extension's own directory, where a released build keeps the binary it ships.
 * The promise rejects if the binary is missing, if the server exits first, or if
 * token is cancelled; in every one of those cases nothing is left running.
 */
export function start(root: string, home: string | undefined, log: vscode.OutputChannel, token: vscode.CancellationToken): Promise<Running> {
  const cfg = vscode.workspace.getConfiguration('depphunter', vscode.Uri.file(root));
  const bin = binaryFor(cfg.get<string>('path'), home);
  const args = argv(cfg, root);

  log.appendLine(`> ${bin} ${args.join(' ')}`);
  const child = spawn(bin, args, { cwd: root });

  return new Promise<Running>((resolve, reject) => {
    let settled = false;
    const stop = (fn: () => void) => {
      if (settled) return;
      settled = true;
      subscription.dispose();
      fn();
    };

    const subscription = token.onCancellationRequested(() => stop(() => {
      child.kill();
      reject(new vscode.CancellationError());
    }));

    // The server logs a line at a time; only whole lines are looked at, so a line
    // split across two reads is not missed and half a line is not matched twice.
    let pending = '';
    const read = (chunk: Buffer) => {
      const text = chunk.toString();
      log.append(text);
      if (settled) return;
      pending += text;
      const lines = pending.split('\n');
      pending = lines.pop() ?? '';
      for (const line of lines) {
        const found = READY.exec(line);
        if (found) {
          stop(() => resolve({ url: found[1], child }));
          return;
        }
      }
      if (pending.length > 8192) pending = ''; // not a line; nothing to wait for
    };
    child.stdout?.on('data', read);
    child.stderr?.on('data', read);

    child.on('error', (err: NodeJS.ErrnoException) => stop(() => reject(
      err.code === 'ENOENT'
        ? new StartError(`${bin} was not found.`, true)
        : new StartError(`${bin} could not be started: ${err.message}`))));

    child.on('exit', (code, signal) => stop(() => reject(new StartError(
      `${bin} stopped before it was serving (${signal ?? `exit code ${code}`}).`))));
  });
}

/**
 * The command line for a server of root. A setting only turns into a flag when it
 * asks for something other than what depphunter would do anyway, so that a folder's
 * .depphunter.yaml, which flags override, still has its say over everything the
 * user has not set here: an enum left at "default", a number left empty, a switch
 * left where depphunter's own default puts it.
 */
export function argv(cfg: vscode.WorkspaceConfiguration, root: string): string[] {
  const args = ['--no-open', '--addr', '127.0.0.1:0'];
  for (const origin of FRAME_ORIGINS) args.push('--embed', origin);

  const text = (key: string, flag: string) => {
    const v = (cfg.get<string>(key) ?? '').trim();
    if (v && v !== 'default') args.push(flag, v);
  };
  const number = (key: string, flag: string) => {
    const v = cfg.get<number | null>(key);
    if (typeof v === 'number' && Number.isFinite(v)) args.push(flag, String(Math.trunc(v)));
  };
  const list = (key: string, flag: string) => {
    for (const v of cfg.get<string[]>(key) ?? []) if (v.trim()) args.push(flag, v);
  };
  // depphunter's switches only go one way each: --watch and the like turn something
  // on that is off by default, --no-cache and the like turn off something that is on.
  const on = (key: string, flag: string) => { if (cfg.get<boolean>(key) === true) args.push(flag); };
  const off = (key: string, flag: string) => { if (cfg.get<boolean>(key) === false) args.push(flag); };

  text('config', '--config');
  on('watch', '--watch');

  list('exclude', '--exclude');
  list('private', '--private');
  list('trustIndexes', '--trust-index');
  number('maxFileSize', '--max-file-size');
  number('resolveDepth', '--resolve-depth');
  on('online', '--online');
  on('explain', '--explain');
  off('cache', '--no-cache');
  off('history', '--no-history');
  number('historyCommits', '--history-commits');

  text('style', '--style');
  text('theme', '--theme');
  text('colorBy', '--color-by');
  text('heightScale', '--height-scale');
  number('expandDepth', '--expand-depth');
  on('showStd', '--show-std');

  list('findings', '--findings');
  off('vulns', '--no-vulns');
  off('links', '--no-links');

  on('lsp', '--lsp');
  text('lspTimeout', '--lsp-timeout');

  const template = editorTemplate(cfg);
  if (template) args.push('--editor', template);

  args.push(...(cfg.get<string[]>('args') ?? []));
  args.push(root);
  return args;
}
