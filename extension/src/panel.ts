// The map in a tab of the editor's own, rather than in its built-in browser.
//
// The built-in browser would do, and did, but it puts the page it shows in a sandbox
// that withholds the pointer lock, and walk mode is looking around with the mouse.
// The editor grants a webview `allow-pointer-lock`; the built-in browser then takes
// it away again on the iframe it draws the page in, and a sandbox can only ever be
// narrowed on the way down, so there is nothing the page itself can do about it.
//
// A panel of our own is the same arrangement without that one subtraction. The iframe
// is written here, and it asks for no sandbox of its own, which means it keeps exactly
// what the editor granted the webview and nothing more. Everything else the built-in
// browser offered - an address bar, back and forward - is chrome for browsing, and
// there is one page here.

import * as vscode from 'vscode';

const panels = new Map<string, vscode.WebviewPanel>();

/**
 * Shows the map for root, in the tab it already has or in a new one.
 * Implements: REQ-EXT-020, REQ-EXT-021
 */
export function open(root: string, title: string, address: string): void {
  let panel = panels.get(root);
  if (!panel) {
    panel = vscode.window.createWebviewPanel('depphunter.map', title, vscode.ViewColumn.Active, {
      enableScripts: true,
      enableForms: true,
      // Without this the editor throws the webview away whenever its tab is not the
      // one on top, and coming back would mean loading and drawing the map again.
      retainContextWhenHidden: true,
    });
    panel.onDidDispose(() => {
      if (panels.get(root) === panel) panels.delete(root);
    });
    panels.set(root, panel);
  }
  panel.title = title;
  panel.webview.html = page(address); // a restart hands over a new port and a new token
  panel.reveal();
}

export function close(root: string): void {
  panels.get(root)?.dispose();
}

export function closeAll(): void {
  for (const panel of [...panels.values()]) panel.dispose();
}

/**
 * The whole of the webview: one iframe holding the map.
 *
 * The iframe deliberately carries no sandbox attribute. A nested frame is already
 * bound by every restriction its ancestors carry, so leaving it off does not remove
 * anything - it only declines to add more, which is what keeps the pointer lock the
 * editor granted. Adding `sandbox` here, even spelled out in full, is how this breaks.
 * Implements: REQ-EXT-021
 */
function page(address: string): string {
  const origin = new URL(address).origin;
  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta http-equiv="Content-Security-Policy"
      content="default-src 'none'; style-src 'unsafe-inline'; frame-src ${attr(origin)};">
<title>depphunter</title>
<style>
  html, body { margin: 0; padding: 0; height: 100%; overflow: hidden; background: var(--vscode-editor-background); }
  iframe { display: block; width: 100%; height: 100%; border: 0; }
</style>
</head>
<body>
<iframe src="${attr(address)}" allow="local-network-access"></iframe>
</body>
</html>`;
}

const attr = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/"/g, '&quot;');
