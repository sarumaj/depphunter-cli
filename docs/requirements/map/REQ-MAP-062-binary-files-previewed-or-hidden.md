---
id: REQ-MAP-062
title: Binary files previewed or kept hidden
scope: map
type: functional
priority: should
status: implemented
verification:
  - unit
  - integration
---

## Statement

The side panel **shall** show a picture, a clip or a recording on the map
(`png`, `jpg`, `jpeg`, `gif`, `webp`, `bmp`, `ico`, `avif`; `mp4`, `m4v`,
`webm`, `ogv`, `mov`; `mp3`, `wav`, `ogg`, `oga`, `flac`, `m4a`) as itself
rather than as text, and **shall** fall back to the binary notice below when the
browser cannot play it. For any other file with a NUL byte in its first 8000
bytes it **shall not** show the content: it **shall** say that the file is
binary and how large, and offer a button, marked as a warning, that shows its
first 64 KB as a hex dump.

The server **shall** answer a request for such a file's source with 415 and
the file's type in `X-Depphunter-Binary`, and **shall** serve a file's bytes
(`?as=raw`) only for files on the map, under one of the media types above or as
`application/octet-stream`, with `Content-Security-Policy: default-src 'none';
sandbox`, honouring byte ranges, and up to 64 MB for media and 4 MB otherwise.

## Rationale

A binary file's bytes decoded as text are noise at best and megabytes of it at
worst. The pictures and clips a repository carries are worth seeing; the rest
is worth knowing about without being shown. Bytes from a repository served on
the map's own origin must not be able to run there.

## Acceptance criteria

1. A PNG on the map is shown as a picture of its own size.
2. A binary file shows a notice and no content until the button is pressed,
   then its first bytes as offset, hexadecimal and printable characters.
3. Text files are served and shown as before.
4. Raw bytes are sandboxed, typed as above, and not served for files off the
   map.
