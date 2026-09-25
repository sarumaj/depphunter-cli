---
id: REQ-SRV-004
uuid: 06218b37-cdae-44b8-8358-9e12e19544fd
title: Configuration endpoint
scope: srv
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M1
verification:
  - integration
---

## Statement

The server **shall** answer `GET /api/config` with the effective view settings
and the capabilities of the run: whether an editor command is configured, the
analyzed root, whether watch mode, language-server references and findings are
enabled, and the name of the project configuration file.

## Rationale

The page needs to know, before drawing anything, which features the server
offers and how the view was configured.

## Acceptance criteria

1. `GET /api/config` returns JSON carrying the `ui` settings and the fields
   `editor`, `root`, `watch`, `configFile`, `lsp` and `findings`.
2. After a settings save the endpoint returns the saved values.
