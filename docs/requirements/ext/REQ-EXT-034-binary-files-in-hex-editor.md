---
id: REQ-EXT-034
title: Binary files open in a hex editor
scope: ext
type: functional
priority: should
status: implemented
verification:
  - unit
  - integration
---

## Statement

For a file the details panel has found to be binary, and that is not a picture,
a clip or a recording, the panel's open button **shall** offer a hex editor
instead of the editor. Asked to, the server **shall** hand the file, as an
`open` event, to the event streams that said they can open one
(`/api/events?opens=hex`) and to no other, answering 202; with none listening
it **shall** open the file in the configured editor as it would any other and
say so in `X-Depphunter-Opened: as-is`. The VS Code extension **shall** listen
that way and open the file in the Hex Editor (`ms-vscode.hexeditor`), offering to
install it, or to open the file as it is, when it is missing. depphunter
**shall not** itself write to the repository.

## Rationale

A binary file opened as text is noise; its bytes and their text side by side is
what editing one means. The editor already has a maintained hex editor, and the
map staying read-only keeps anything it shows from being able to change the
repository.

## Acceptance criteria

1. A binary file's open button reads "Hex editor", and `O` does the same.
2. With the extension attached, the file opens in the Hex Editor, and the map's
   pages are not handed it; the editor command does not run as well.
3. Without the Hex Editor installed, the extension offers to install it or to
   open the file as it is.
4. Without the extension, the file opens as it is and the status line says how
   to reopen it in the Hex Editor.
5. Files off the map are refused, hex or not.
