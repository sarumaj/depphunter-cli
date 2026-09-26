// The depphunter the extension runs, on the PATH of the editor's own terminals.
//
// A released build carries its binary inside the extension's directory, where no
// shell would look for it - so installing the extension gives the map but not the
// command, and `depphunter --export html` in the terminal beside it says "not found"
// or, worse, finds some other version. The editor lets an extension adjust the
// environment of every terminal it opens; this puts the binary's folder first on
// PATH there, and nowhere else: not the shell profile, not the system.

import * as path from 'node:path';
import type * as vscode from 'vscode';

import { binaryFor } from './binary';

/**
 * The folder to put on PATH, or undefined for none: when it is switched off, and when
 * the binary is a bare name, which the shell already finds on the PATH it has - the
 * universal build and a build from a checkout, which carry no binary of their own.
 * Implements: REQ-EXT-033
 */
export function binDirFor(configured: string | undefined, home: string | undefined, enabled = true): string | undefined {
  if (!enabled) return undefined;
  const bin = binaryFor(configured, home);
  return path.isAbsolute(bin) ? path.dirname(bin) : undefined;
}

/**
 * Makes the terminals' PATH start with `dir`, replacing whatever this extension put
 * there before; undefined takes it off again. The editor keeps the change across
 * reloads, applies it to every terminal opened from now on, and offers to relaunch
 * the ones already open.
 * Implements: REQ-EXT-033
 */
export function exposeOnPath(env: vscode.EnvironmentVariableCollection, dir: string | undefined): void {
  env.clear();
  if (!dir) return;
  env.prepend('PATH', dir + path.delimiter);
  // Shown where the editor says why a terminal's environment changed. Newer editors
  // only; on older ones the property is simply not read.
  (env as { description?: string }).description = `Puts the depphunter the extension runs (${dir}) on PATH`;
}
