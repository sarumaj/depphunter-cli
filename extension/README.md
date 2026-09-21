# depphunter for VS Code

Browse the folder you are working in as an interactive isometric city, in a tab
beside the code.

The extension is a thin wrapper: it starts `depphunter` for the folder, waits for
it to say where it is listening, and opens that address in the editor's built-in
browser. The map is the same one `depphunter` serves anywhere else - clicking,
walking, the panel, the findings and the live updates all work as they do in a
browser tab.

## Requirements

The `depphunter` binary, from
[the releases page](https://github.com/sarumaj/depphunter-cli/releases) or
`go install github.com/sarumaj/depphunter-cli/cmd/depphunter@latest`. It has to
be on `PATH`, or named by the `depphunter.path` setting.

## Use

`depphunter: Open the Map` from the command palette, or right-click a folder in
the explorer and choose it there. The first run analyzes the folder, which takes
a moment on a large repository; after that the tab opens straight away: the
server stays up until the window closes or you run `depphunter: Stop the
Server`.

With `depphunter.watch` on (the default), the map follows your edits: the server
re-analyzes when files change and the page redraws itself.

The map's **Open in editor** button opens the file at the line you were looking
at, in this editor. The extension finds this editor's own command-line launcher
and hands it to the server; `depphunter.editorCommand` overrides that if you want
the file somewhere else.

## Settings

| Setting                    | Default         | What it does                                    |
|----------------------------|-----------------|-------------------------------------------------|
| `depphunter.path`          | `depphunter`    | The binary. A bare name is looked up on `PATH`. |
| `depphunter.watch`         | `true`          | Re-analyze on file changes and update the map.  |
| `depphunter.style`         | `default`       | `city`, `circuit` or `galaxy`.                  |
| `depphunter.findings`      | `[]`            | Scanner reports to place on the map.            |
| `depphunter.args`          | `[]`            | Further arguments, one per entry.               |
| `depphunter.openIn`        | `simpleBrowser` | The built-in browser, or your own.              |
| `depphunter.editorCommand` | `""`            | What **Open in editor** runs.                   |

Anything the settings do not cover goes in `depphunter.args`, or in the project's
`.depphunter.yaml`, which the server reads as usual. Run `depphunter --help` for
the full list.

## How the framing works

A page in the built-in browser is inside a webview, which means it is framed by an
origin belonging to the editor. depphunter refuses to be framed by default, so the
extension starts it with `--embed`, naming the two origins an editor webview can
have: `vscode-webview:` in the desktop editor and `https://*.vscode-cdn.net` in
the browser build. Nothing else may frame it - a page at any other origin is
refused by the browser before it loads.

In that mode the session token stays in the address instead of being exchanged for
a cookie, because a cookie set by the map would be a third-party cookie inside the
frame and would never be sent back. This is why the extension reads the address
out of the server's own output rather than putting it together from the port: that
address is the only place the token appears.

## Remote workspaces

Over SSH, WSL and dev containers the port is forwarded to `localhost` on your
machine and everything works. In Codespaces and on vscode.dev the forwarded
address is a public hostname, which the server rejects as a DNS-rebinding
attempt: it only answers to `localhost` and `127.0.0.1`. Run `depphunter` in a
terminal there instead, for now.

## Publishing

The extension is not on any marketplace yet. What it takes, when it is time:

1. `npm install -g @vscode/vsce`, and `vsce package` to build `depphunter-0.1.0.vsix`.
   That file can be installed directly - *Extensions: Install from VSIX…* - which
   is enough for sharing it without publishing anything.
2. For the **Visual Studio Marketplace**: create an Azure DevOps organization,
   then a personal access token with *Marketplace → Manage* scope for **all
   accessible organizations**. Create the publisher at
   <https://marketplace.visualstudio.com/manage>, whose ID has to match the
   `publisher` field in `package.json`. Then `vsce login <publisher>` with that
   token, and `vsce publish` (or `vsce publish minor` to bump the version first).
3. For **Open VSX**, which is what VSCodium, Cursor, Gitpod and Eclipse Theia
   install from: an account at <https://open-vsx.org>, an access token, and
   `npx ovsx publish depphunter-0.1.0.vsix -p <token>`.

The extension carries no binary: it runs whatever `depphunter` it finds, so one
VSIX works on every platform and it does not need per-platform targets. If a
binary is ever bundled instead, that changes - the VSIX would have to be built
once per `--target` (`win32-x64`, `linux-x64`, `darwin-arm64`, …).

A 128×128 PNG `icon.png` beside `package.json`, named by an `"icon"` field, is
worth adding before publishing: without one the marketplace shows a placeholder.

## Building it

```sh
npm install
npm run compile                              # or: npm run watch
DEPPHUNTER=../path/to/depphunter npm test    # skipped without a binary to test
```

Then open this directory in VS Code and press F5, which launches a second window
with the extension loaded.

The tests run the built extension against a real server with the editor stubbed
out (`test/stub.js`). That is where the coupling is: the arguments the extension
starts `depphunter` with, and the address it reads back out of its output.
Neither the Go tests nor the type checker see either one.
