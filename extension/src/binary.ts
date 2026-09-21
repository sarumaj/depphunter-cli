// Which depphunter to run.
//
// A released VSIX carries the binary for the platform it was built for, so the
// extension and the server it starts are one version, pinned together, and there is
// nothing to install alongside it. A build from a checkout carries none, and falls
// back to whatever is on the PATH.
//
// The setting wins over both. It is the only way to point the extension at a build
// that is not the one it shipped with - a local one being worked on, or a newer
// release on a machine where the extension has not been updated - so an explicit
// value is never second-guessed.

import * as fs from 'node:fs';
import * as path from 'node:path';

/** Where a released VSIX puts its binary, relative to the extension's own directory. */
const BUNDLED = 'bin';

/**
 * The binary this build ships, or undefined when it ships none - which is every
 * build that did not come from a release.
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

/** What to start: the setting if it names something, else what we ship, else the PATH. */
export function binaryFor(configured: string | undefined, home: string | undefined): string {
  return configured?.trim() || bundled(home) || 'depphunter';
}
