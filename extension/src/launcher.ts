// Where this editor's own command-line launcher is.
//
// depphunter opens a file by running a command template - the server does it, not the
// page - and it can work one out for itself. But what it finds is whatever is on the
// PATH of the process the extension host spawned, which need not be this editor, or
// this window's editor, or anything at all. So the extension tells it, and only lets
// depphunter guess when there is nothing here to point at.

import * as fs from 'node:fs';
import * as path from 'node:path';
import * as vscode from 'vscode';

/**
 * The --editor template to pass, or undefined to leave the choice to depphunter.
 * A template configured by the user wins: it is the only way to name an editor that
 * is not this one.
 * Implements: REQ-EXT-025
 */
export function editorTemplate(cfg: vscode.WorkspaceConfiguration): string | undefined {
  const set = (cfg.get<string>('editorCommand') ?? '').trim();
  if (set) return set;
  const cli = launcher();
  return cli ? `${quote(cli)} -g {file}:{line}` : undefined;
}

function launcher(): string | undefined {
  if (process.platform === 'win32') {
    // On Windows the launcher in bin/ is a .cmd, and a .cmd needs a shell to run it:
    // depphunter starts the editor without one, on purpose, so that a file name can
    // never be read as a command. The application takes the same arguments, and in
    // the desktop editor the extension host is running on it.
    const exe = process.execPath;
    return path.basename(exe).toLowerCase() === 'node.exe' ? undefined : exe;
  }
  // Everywhere else it is a shell script beside the application: appRoot is
  // .../resources/app, and the script is in its bin directory under the name
  // product.json gives it - "code", "code-insiders", "codium", "cursor", ...
  const root = vscode.env.appRoot;
  if (!root) return undefined;
  let name = 'code';
  try {
    const product = JSON.parse(fs.readFileSync(path.join(root, 'product.json'), 'utf8')) as unknown;
    const named = (product as { applicationName?: unknown }).applicationName;
    if (typeof named === 'string' && named) name = named;
  } catch {
    // Not there, or not readable: "code" is what it is called nearly everywhere.
  }
  const full = path.join(root, 'bin', name);
  return fs.existsSync(full) ? full : undefined;
}

// depphunter splits the template with POSIX shell quoting rules, so a path with a
// space in it has to be quoted - and on Windows, where it is all backslashes, it has
// to be quoted whether it has a space in it or not.
function quote(s: string): string {
  return `'${s.replace(/'/g, `'\\''`)}'`;
}
