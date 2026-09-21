# depphunter for VS Code

Browse the folder you are working in as an interactive isometric city, in a tab
beside the code.

The extension is a thin wrapper: it starts `depphunter` for the folder, waits for
it to say where it is listening, and opens that address in the editor's built-in
browser. The map is the same one `depphunter` serves anywhere else - clicking,
walking, the panel, the findings and the live updates all work as they do in a
browser tab.

Its manifest lives at the root of the repository, not here, so that it shares the
project's `README.md` and `LICENSE` instead of keeping copies: `package.json`,
`tsconfig.json` and `.vscodeignore` are up there, and this directory holds the
source, the tests and this page. `.vscodeignore` is written the other way round
from usual - it leaves everything out and lets the handful of extension files
back in - because most of what is in this repository is a Go program.

## Install

Every [release](https://github.com/sarumaj/depphunter-cli/releases) carries a
`depphunter_<version>_vscode.vsix` beside the binaries. Download it, then in VS
Code: *Extensions: Install from VSIX…* from the command palette, and pick the
file. Or from a terminal:

```sh
code --install-extension depphunter_1.2.3_vscode.vsix
```

It also needs the `depphunter` binary itself - from the same release, or
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

1. `npm install -g @vscode/vsce`, and `vsce package` at the root of the
   repository to build the `.vsix`. The release workflow already does this and
   attaches the file to every release, packaged with the tag's version; the
   `version` in `package.json` is only what a build from a checkout gets.
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

The marketplace page is the project's own `README.md`, with its relative links
rewritten to GitHub by `vsce`. `vsce package --readme-path extension/README.md`
would put this page there instead, if the project page ever reads badly as an
extension listing.

A 128×128 PNG `icon.png` at the root, named by an `"icon"` field in
`package.json`, is worth adding before publishing: without one the marketplace
shows a placeholder.

## Working on it

An extension is a Node program the editor loads into a process of its own, the
*extension host*. It is not a web page, it has no DOM, and `console.log` from it
does not go where you might expect. Everything below follows from that.

### The first run

From the root of the repository:

```sh
npm install
npm run compile
```

Open the repository as the workspace, press F5, and pick **Run the VS Code
extension** if you are asked. A second editor window opens, titled *[Extension
Development Host]*. That window has your extension loaded and nothing else
different about it; the first window is now a debugger attached to it.

In the new window, open a folder with some code in it and run
`depphunter: Open the Map` from the command palette (Ctrl/Cmd+Shift+P).

### Changing code

`npm run watch` in a terminal recompiles on every save. The extension host does
not reload itself, so after a save go to the *[Extension Development Host]*
window and run **Developer: Reload Window** (Ctrl/Cmd+R). That is the whole
edit-run loop.

Reloading kills the extension host, which kills the `depphunter` it started, so
the next open analyzes again from a warm cache. Nothing leaks between runs.

### Breakpoints

Click the gutter beside a line in `extension/src/*.ts` in the **first** window
and it will be hit - `tsc` writes source maps, so you are stopped in the
TypeScript, not in `extension/out/`. The Debug Console there is where
`console.log` from the extension goes, and where an uncaught exception is
reported.

Useful places to stop when something is wrong: `start()` in `server.ts` (what
arguments went to the binary), the `read` function just below it (what came
back), and `show()` in `extension.ts` (what address the browser was handed).

### The four places output goes

This is the part that catches people out. There are four separate consoles and
they show different things:

| Where                       | What is in it                                                                | How to open it                                                                                              |
|-----------------------------|------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------|
| Debug Console, first window | `console.log` and exceptions from *your* extension                           | F5 opens it                                                                                                 |
| Output → **depphunter**     | the server's own log, verbatim: the command line, the analysis, the warnings | *Output: Focus on Output View*, then pick depphunter in the dropdown - or `depphunter: Show the Server Log` |
| Output → **Extension Host** | the editor's own complaints about loading extensions                         | same dropdown                                                                                               |
| Webview developer tools     | errors from the map itself - WebGL, the page's JavaScript                    | **Developer: Open Webview Developer Tools** in the *[Extension Development Host]* window                    |

If the map opens but is blank or broken, it is the last one you want. If the map
never opens, it is the first two.

### When it goes wrong

- **"depphunter was not found"** - the extension host inherited a `PATH` without
  it. Set `depphunter.path` to the absolute path; that always works.
- **The tab opens empty, and the webview console says *Refused to frame*** - the
  server was started without the right `--embed` origin. The **depphunter**
  output channel shows the exact command line it used; check it has
  `--embed vscode-webview:` in it.
- **Nothing happens at all and there is no error** - the extension may not have
  activated. **Developer: Show Running Extensions** in the development window
  lists what loaded and how long each took.
- **A change did nothing** - `npm run watch` was not running, or the window was
  not reloaded. The timestamp on `extension/out/extension.js` settles it.

### Tests

```sh
go build -o depphunter ./cmd/depphunter
DEPPHUNTER="$PWD/depphunter" npm test   # skipped without a binary to test
```

These run without an editor at all: `extension/test/stub.js` stands in for the
editor API, so the built `extension/out/extension.js` drives a real server and
the test checks what came back. That is where the coupling is - the arguments
the extension starts `depphunter` with, and the address it reads out of its
output. Neither the Go tests nor the type checker see either one.

Give `DEPPHUNTER` an absolute path: the extension starts the binary in the folder
it is mapping, and a relative one is not resolved the same way on every platform.

### Installing your build

```sh
npx @vscode/vsce package                  # depphunter-0.1.0.vsix, at the root
code --install-extension depphunter-0.1.0.vsix
```

That installs it into your real editor, not the development window. `code
--uninstall-extension sarumaj.depphunter` removes it again.
