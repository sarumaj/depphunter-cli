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

import { editorTemplate } from './launcher';

// The origins the map may be framed by. A webview is given an origin of its own for
// every session - vscode-webview://<uuid> in the desktop editor, and
// https://<uuid>.vscode-cdn.net in the browser build - so there is no exact name to
// give, only the scheme and the wildcard host.
const FRAME_ORIGINS = ['vscode-webview:', 'https://*.vscode-cdn.net'];

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
 * Starts a server for root and resolves once it says where it is listening. The
 * promise rejects if the binary is missing, if the server exits first, or if token
 * is cancelled; in every one of those cases nothing is left running.
 */
export function start(root: string, log: vscode.OutputChannel, token: vscode.CancellationToken): Promise<Running> {
  const cfg = vscode.workspace.getConfiguration('depphunter', vscode.Uri.file(root));
  const bin = cfg.get<string>('path')?.trim() || 'depphunter';
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

function argv(cfg: vscode.WorkspaceConfiguration, root: string): string[] {
  const args = ['--no-open', '--addr', '127.0.0.1:0'];
  for (const origin of FRAME_ORIGINS) args.push('--embed', origin);
  if (cfg.get<boolean>('watch')) args.push('--watch');

  const style = cfg.get<string>('style');
  if (style && style !== 'default') args.push('--style', style);

  for (const report of cfg.get<string[]>('findings') ?? []) {
    if (report.trim()) args.push('--findings', report);
  }

  const template = editorTemplate(cfg);
  if (template) args.push('--editor', template);

  args.push(...(cfg.get<string[]>('args') ?? []));
  args.push(root);
  return args;
}
