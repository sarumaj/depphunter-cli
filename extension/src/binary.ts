// Which depphunter to run.
//
// A released VSIX carries the binary for its platform, so the extension and the
// server it starts are one pinned version with nothing to install alongside. A build
// from a checkout carries none and falls back to the PATH. The setting wins over
// both, and is never second-guessed: it is the only way to point at a build that is
// not the one that shipped.

import * as fs from 'node:fs';
import * as path from 'node:path';

/** Where a released VSIX puts its binary, relative to the extension's own directory. */
const BUNDLED = 'bin';

/**
 * The binary this build ships, or undefined when it ships none - which is every
 * build that did not come from a release.
 * Implements: REQ-EXT-019
 */
export function bundled(home: string | undefined): string | undefined {
  if (!home) return undefined;
  const file = path.join(home, BUNDLED, process.platform === 'win32' ? 'depphunter.exe' : 'depphunter');
  if (!fs.existsSync(file)) return undefined;
  if (process.platform !== 'win32') {
    // A VSIX is a zip, and a zip's permission bits do not survive every installer,
    // so the file can arrive without its executable bit. Setting it here costs
    // nothing and beats finding out when the server will not start.
    try {
      fs.chmodSync(file, 0o755);
    } catch {
      // A read-only install directory, where it is already whatever it is.
    }
  }
  return file;
}

/**
 * What to start: the setting if it names something, else what we ship, else the PATH.
 * Implements: REQ-EXT-019
 */
export function binaryFor(configured: string | undefined, home: string | undefined): string {
  return configured?.trim() || bundled(home) || 'depphunter';
}

/**
 * Why binaryFor's choice could not be started, saying where it was looked for: the
 * setting's binary, or - with none set - this build's own and then the PATH. A build
 * that ships no binary (one from a checkout, or the universal package) says so, which
 * is the difference between installing depphunter and installing the right package;
 * a window left running the version an update replaced is told to reload.
 * Implements: REQ-EXT-019
 */
export function notFound(configured: string | undefined, home: string | undefined): string {
  const bin = binaryFor(configured, home);
  if (configured?.trim()) return `${bin} was not found (the depphunter.path setting).`;
  // An update installs the new version beside the old and removes the old once no
  // window needs it - but a window opened before the update still runs the old one,
  // and its directory may already be gone.
  if (home && !fs.existsSync(home)) {
    return `depphunter was not found: this window still runs a version of the extension that has since been updated or removed (${home}). Reload the window.`;
  }
  if (bin !== 'depphunter') return `${bin} was not found.`;
  const own = home ? path.join(home, BUNDLED) : 'its bin directory';
  return `depphunter was not found: this build of the extension carries none in ${own}, and there is none on the PATH.`;
}
