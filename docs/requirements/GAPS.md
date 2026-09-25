# Requirements review: gaps and discrepancies

This document records the review of [docs/REQUIREMENTS.md](../REQUIREMENTS.md)
against the code (September 2026) that produced this specification. It lists:

1. requirements that are not, or only partly, implemented;
2. defects found while tracing requirements to the code;
3. places where the design log and the code disagree;
4. behavior implemented in the code that no requirement covers;
5. gaps in automated verification.

The live counts are in [TRACEABILITY.md](TRACEABILITY.md), which
`node tools/reqtrace.mjs` regenerates. This document is written by hand and
is updated when an item here is resolved.

## 1. Requirements not met

The review found 2 requirements not implemented and 17 partly implemented.
All were resolved in follow-up changes, either by repairing the code or, where
the code reflects a later decision than the design log or the design log's
target could not be checked, by amending the requirement. Each requirement's
Notes record what was done.

### Repaired in the code

| Requirement | Finding | Repair |
|-------------|---------|--------|
| [REQ-EXP-015](exp/REQ-EXP-015-backpack-export-offered-by-the-page.md) | The page's backpack had no export; only the extension offered it. | The backpack panel offers Markdown, CSV and JSON downloads from `GET /api/backpack?format=`, hidden in the static export and while the backpack is empty. |
| [REQ-MAP-004](map/REQ-MAP-004-collapsed-directory-height-follows-mean-file.md) | **Defect.** Every collapsed directory was drawn at the floor height: `layout.js` read `totalBulk`, which `computeVisibility` did not return (regression of 1d979b8). | `computeVisibility` sums `totalBulk`; `web/uitest/layout.test.mjs`. |
| [REQ-WALK-006](walk/REQ-WALK-006-step-height-and-collisions.md) | `STEP = 0.32` exceeded a terrace wall (0.28), so terrace walls were walked up. | `STEP = 0.15` (half a storey); the wading allowance was raised so the walker still climbs out of the water. |
| [REQ-AUTH-010](auth/REQ-AUTH-010-encrypted-password-left-alone.md) | **Security.** A Maven encrypted password was sent as ciphertext. | Encrypted `{…}` passwords are skipped; the test now covers a mirror naming the encrypted server. |
| [REQ-SUP-026](sup/REQ-SUP-026-oci-base-image-via-manifest-config.md) | Only Docker Hub images were asked for their base image. | A registry named by the machine's container configuration (`auths`, `credHelpers`) or by `--trust-index` is asked; one only the repository names stays marked. |
| [REQ-CFG-006](cfg/REQ-CFG-006-environment-variable-per-scalar-setting.md) | `ui.tool` had no environment variable. | `DEPPHUNTER_TOOL`. |
| [REQ-DIST-006](dist/REQ-DIST-006-license-notices-ship-with-releases.md) | Release archives lacked the Go modules' license notices. | `scripts/dist.sh` copies every vendored `LICENSE*`, `COPYING*` and `NOTICE*` to `licenses/go/<module>/`. |
| [REQ-CS-004](cs/REQ-CS-004-case-insensitive-package-ids.md) | Package map and central versions were case-sensitive. | Both keyed by lower-case id. |
| [REQ-CI-013](ci/REQ-CI-013-unversioned-references-flagged-floating.md) | A versionless component was neither pinned nor floating; an untagged image got `latest`. | Both float with no invented version; the index still asks for `latest`. [REQ-CI-012](ci/REQ-CI-012-only-digests-pin-images.md) was aligned. |
| [REQ-MD-014](md/REQ-MD-014-web-link-answers-cached.md) | Web-link answers were cached for 6 h. | Own 24 h TTL; OSV keeps 6 h. |
| [REQ-MAP-014](map/REQ-MAP-014-each-color-encodes-one-quantity-with.md) | The legend did not explain floating and unresolved packages. | A Packages section in the legend. |
| [REQ-MAP-019](map/REQ-MAP-019-fit-and-reset-view-actions.md) | No reset action. | Reset button and `R` in the map view. |
| [REQ-MAP-052](map/REQ-MAP-052-style-environment-colors-from-the-stylesheet.md) | Circuit and galaxy colors overrode both themes. | Separate light and dark environment values per style. |
| [REQ-UI-004](ui/REQ-UI-004-first-visit-introduction-of-five-cards.md) | Introduction card described the removed roads. | Card describes arcs, dimming and the panel; test added. |
| [REQ-UI-014](ui/REQ-UI-014-menus-wait-for-the-pointer-lock.md) | Menus did not wait for the pointer lock release. | `whenUnlocked` in `dom.js`, used by the Filters and Export menus, the backpack and the photographs; `web/uitest/menus.test.mjs`. |

### Requirement amended

| Requirement | Finding | Amendment |
|-------------|---------|-----------|
| [REQ-TOOL-008](tool/REQ-TOOL-008-hand-from-pinned-upstream-model.md) | M14 had the hand grown with the Skin modifier; `tools/hand.py` prepares the WebXR `generic-hand`, and its docstring records why. | The requirement specifies the pinned upstream model; the UI still downloads nothing. |
| [REQ-TOOL-016](tool/REQ-TOOL-016-camera-build.md) | M14 had the camera held in both hands; since M21 the left hand carries the secondary tool. | The camera is held in the right hand. |
| [REQ-HUNT-010](hunt/REQ-HUNT-010-bug-per-finding.md) | M13 had a bug for every finding; the code caps bugs at 140 and draws reachable vulnerabilities as fires. | The cap and the fires are specified; every finding stays listed in the panel. |
| [REQ-LANG-029](lang/REQ-LANG-029-cold-analysis-time.md) | "Under 5 s for 10k files" named no machine. On 4 cores the reference project took 8.9 s, nearly all of it tree-sitter parsing at the runtime's own speed. | Stated for an 8-core reference machine, with a CPU budget (36 CPU-seconds) and a parallel-efficiency floor (0.85 × N) that can be checked anywhere, measured by `BenchmarkColdAnalysis`. Measured: 34.3 CPU-seconds and a 3.67× speed-up on 4 cores; about 4.7 s projected for 8 cores. |

## 2. Defects

Found while tracing, and fixed in the same follow-up change:

1. Collapsed directories were flat
   ([REQ-MAP-004](map/REQ-MAP-004-collapsed-directory-height-follows-mean-file.md)).
2. Maven encrypted passwords were sent
   ([REQ-AUTH-010](auth/REQ-AUTH-010-encrypted-password-left-alone.md)).
3. Fallback trees were invisible but solid: the fallback `TREES` table in
   `city.js` used `trunk`/`crown` where `makeProps` reads `stem`/`head`
   ([REQ-CITY-022](city/REQ-CITY-022-vegetation-geometry.md);
   `web/uitest/props.test.mjs`).
4. `psgallery` was not an ecosystem prefix for `--private`
   ([REQ-SUP-035](sup/REQ-SUP-035-private-pattern-ecosystem-prefix.md)).
5. Terrace walls could be walked up
   ([REQ-WALK-006](walk/REQ-WALK-006-step-height-and-collisions.md)). At very
   low frame rates (below about 23 fps) the steepest ramp can now stall a
   runner, since one frame's rise exceeds the step; the requirement's Notes
   record this.
6. A stale comment in `walk.js` described `onPhoto` as the result of a double
   use of the camera (obsolete since M28).

## 3. Design log and code disagree

The requirement files state the behavior that the design log intends, except
where a later milestone overrides an earlier one. Where the code differs, the
discrepancy is recorded below and in the requirement's Notes; the decision
whether to change the code or the specification is open.

| Area | Design log | Code |
|------|------------|------|
| User config ([REQ-CFG-002](cfg/REQ-CFG-002-user-config-file-location.md)) | `$XDG_CONFIG_HOME/depphunter/config.yaml` | `os.UserConfigDir`: XDG on Linux, `~/Library/Application Support` on macOS, `%AppData%` on Windows |
| Host check ([REQ-SEC-005](sec/REQ-SEC-005-foreign-host-header-rejected.md)) | Foreign `Host` headers rejected | Checked only when bound to a loopback address |
| CI targets ([REQ-DIST-014](dist/REQ-DIST-014-ci-builds-all-targets-per-push.md)) | Every target built on each push | Pushes to `main` and pull requests only |
| `--ui-default` ([REQ-CFG-016](cfg/REQ-CFG-016-ui-default-seeds.md)) | An unusable seed is refused | A well-formed but invalid value (`theme=neon`) is reported only when it is the value in effect |
| `--config` | A replacement for the project config | Also trusted: it may set `editor`, `online` and `trust_indexes` |
| Parsing bound ([REQ-LANG-011](lang/REQ-LANG-011-per-file-parse-time-bound.md), [REQ-LANG-012](lang/REQ-LANG-012-oversized-and-minified-files-skipped.md)) | "Bounded per file" | 3 s tree-sitter timeout and a fixed 1 MiB parse limit: files between 1 and 2 MiB (the `--max-file-size` default) are counted but never parsed; minified means > 20 000 bytes and > 250 bytes per line |
| Extraction cache key ([REQ-LANG-026](lang/REQ-LANG-026-content-addressed-extraction-cache.md)) | Content, plugin version, extension | Also the plugin name and an optional plugin class (`lang.Classifier`) |
| Renames ([REQ-HIST-006](hist/REQ-HIST-006-history-follows-renames.md)) | M5: `--no-renames`, renames not followed | `-M` since M6; the M5 decision is obsolete |
| Severity ([REQ-FND-018](fnd/REQ-FND-018-one-severity-scale-from-cvss-vectors.md)) | Five levels | Six: `unknown` below `info`; Trivy misconfigurations and secrets keep an uncapped severity word |
| "No reports, no `--online`, shows nothing" ([REQ-FND-020](fnd/REQ-FND-020-findings-placed-where-they-belong.md)) | Nothing shown | Markdown link findings are on by default, so findings can still appear |
| LSP cache ([REQ-LSP-006](lsp/REQ-LSP-006-references-cached-by-content-and-servers.md)) | Cached by content and servers | Partial results are never cached |
| Resolution report ([REQ-TRC-015](trc/REQ-TRC-015-written-report-cuts-lists-and-cells.md)) | The written report cuts long lists | Only the text digest cuts lists (50); the Markdown document keeps every question and cuts cells (200 characters, 72 in text) |
| Go environment ([REQ-SUP-036](sup/REQ-SUP-036-goprivate-and-gonoproxy-read.md)) | GOPRIVATE and GONOPROXY | Also GONOSUMDB and GONOSUMCHECK |
| Version travels ([REQ-SUP-029](sup/REQ-SUP-029-version-travels-with-dependency.md)) | The version travels with each dependency | For npm a range travels, and the next question asks for the `latest` document |
| Bite damage ([REQ-WALK-028](walk/REQ-WALK-028-bug-bites.md)) | A critical bite is worth three notes, at most once a second | 34 against 4 (8.5×); `BITE_EVERY` is 1.1 s |
| Fall damage ([REQ-WALK-027](walk/REQ-WALK-027-fall-damage.md)) | More than "about a house" | `SAFE_FALL = 3` units, about ten storeys |
| Pointer glitches ([REQ-WALK-011](walk/REQ-WALK-011-mouse-look-and-glitch-filter.md)) | Jumps of 250 px or more ignored | Clamped to 250 px |
| Dimmed boxes ([REQ-CITY-005](city/REQ-CITY-005-dimmed-boxes-drawn-faded.md)) | Drawn plain, without windows | Facades kept and blended 72 % towards the plain color |
| Street grid | Built only when walk mode is shown | Built for every layout, since M10 put streets in the isometric view |
| Pan margin ([REQ-MAP-020](map/REQ-MAP-020-camera-target-clamped-to-the-map.md)) | A quarter of the map size, at least 6 units | 6 + 0.25 × size |
| Walk introduction ([REQ-UI-011](ui/REQ-UI-011-walk-mode-introduction-on-first-walk.md)) | Five topics | Six cards (fire has its own) |
| Catching tools ([REQ-TOOL-026](tool/REQ-TOOL-026-bug-catching-tools.md)) | Three by design, two incidentally | The camera catches as well: six tools |
| Net ([REQ-TOOL-065](tool/REQ-TOOL-065-net-thrown-as-hoop.md)) | M10: throws a spinning hoop | Throws nothing; swung within reach 2.1 |
| `H` ([REQ-TOOL-032](tool/REQ-TOOL-032-h-hides-both-tools.md)) | Stows both tools | Hides their drawing; the tools keep working and shots leave from the eye |
| Fishing rod ([REQ-TOOL-004](tool/REQ-TOOL-004-tool-projectile-models.md)) | Casts a bobber | Its reel also pulls the walker to what it hooked, like a grapple |
| Extension ([REQ-EXT-020](ext/REQ-EXT-020-map-location-setting.md)) | README intro: the map opens in the built-in browser | Default `depphunter.openIn` is `webview`, a dedicated tab |

## 4. Behavior without a requirement

The following behavior is implemented, and in several cases tested, but no
requirement specifies it. Each is a candidate for a new requirement, or for
removal if it is unintended.

### Analysis and resolution

- Go `replace` directives (module, directory, version-specific,
  longest-match); lax `go.mod` fallback; imports of `C` ignored.
- JavaScript specifiers starting with `~`, `#`, `@/`, URLs, `data:` and
  absolute paths are dropped; `tsconfig.json` beats `jsconfig.json`; a yarn
  range with no descriptor falls back to a single locked version.
- Python distribution candidates `python-X`, `pyX` and `X-python`.
- Standard-library ecosystems for Rust and .NET; Java imports under the
  project's own groupId treated as generated code; PowerShell
  `RequiredModules` supplying versions.
- CI: unparseable YAML yields nothing; `$VARIABLE` images skipped; a registry
  port is not a tag; a remote include is named without its query.
- Markdown: root-relative paths, percent-unescaping, `{#id}` and HTML `id`
  anchors; the web check falls back from HEAD to GET, runs 8 workers and sends
  stored credentials; missing files are medium severity, missing anchors low.
- The scanner skips symbolic links and non-regular files, drops duplicate
  `git ls-files` entries and gives duplicate symbols an `@<line>` suffix.
- The extraction cache prunes entries the last complete run did not use.
- Index clients: 30 s timeout, 8 MB response limit, User-Agent, 5-minute
  retry after a failure, NuGet service index cached per run, Go proxy answers
  skip `// indirect`; index sources re-read on every `--watch` analysis.
- Credentials travel over plain HTTP only to loopback or to hosts the
  machine's own configuration names with `http://`.
- `DEPPHUNTER_PRIVATE` and `DEPPHUNTER_TRUST_INDEXES`; a project config cannot
  set `online`.
- Findings: duplicates across tools collapsed; stable 8-byte ids; OSV picks the
  lowest fix on the release line in use; govulncheck findings carry the
  reaching function; findings with unknown paths go to the nearest drawn
  directory.

### Server and watch

- Security headers (CSP, `nosniff`, `X-Frame-Options: DENY`,
  `Referrer-Policy: no-referrer`); per-token cookie names; constant-time token
  comparison; `--embed` mode with the `X-Depphunter-Token` header and a
  `frame-ancestors` policy.
- Request limits: `/api/file` 4 MiB (413), `/api/settings` 64 KB, `/api/open`
  4 KB; `/api/resolution` answers 404 before the first analysis and 400 for an
  unknown format.
- Background datasets carry an ETag and answer 304; an identical re-read is
  not announced; the event stream sends a 25 s heartbeat and `retry: 2000`.
- A change stream that never goes quiet is forced through after ten debounce
  intervals; editor scratch files and chmod-only events are ignored.
- Save is atomic and keeps the file's permissions; `K` saves; the static
  export hides Save.
- `internal/minify` strips comments from served and exported assets; the
  static export inlines the 3D models and the tour pictures.
- `editor.Detect` reads `$VISUAL`/`$EDITOR` and a list of GUI editors,
  excluding terminal editors on purpose.

### Map and walk mode

- The initial depth is the deepest level with at most 600 items (`autoLevel`).
- Tooltip delay of 240 ms; double-clicking open ground enters walk mode;
  `Backspace` selects the parent; revealing a node lifts the filters that hide
  it; the path filter is debounced by 250 ms; fzf switches algorithm above
  50 000 entries; the circuit and galaxy styles redraw on a 66 ms ticker.
- Fires for reachable vulnerabilities (`fires.js`, `flames.js`): they spread,
  hurt the walker, are put out by the extinguisher and pulse on the tracker.
- Health mends 4 points per second after 5 s unharmed; a day/night mode; HUD
  mode chips; a marker for where the walker last stood; walk mode resumes at
  that position.
- Finding pins in the isometric view (`pins.js`); the backpack marks fixed
  findings (at most 500); the photograph stash holds at most 24 and opens with
  `G`; the grapple's 60-unit line and 5 s timeout; the tracker's close-in mode
  below 14 units; reach messages; the wheel greys out tools with an empty tank.

### VS Code extension

- A warning with Show Log and Restart when a server stops on its own; a
  missing-binary error offering the releases page; the executable bit restored
  on the bundled binary; a probe of the map address with hints for 401 and 303;
  a cancellable start notification; a 30 s HTTP timeout; `simpleBrowser`
  falling back to the dedicated tab.
- The release workflow also publishes the extension to the Marketplace and
  Open VSX, and CI rejects Git LFS pointer files.

## 5. Verification gaps

[TRACEABILITY.md](TRACEABILITY.md) lists every live requirement without a
`Verifies:` annotation. The largest gaps:

- **Walk mode and the city** are verified almost entirely by hand: the
  `Walker` class is never instantiated in a test. Health, wind, bridges, the
  tool catalogue, the wheel and the stash are covered by `web/uitest`.
- **The map**: `layout` and `computeVisibility` are now covered
  (`web/uitest/layout.test.mjs`); `globMatcher`, `assignSlots` and the panel
  tree could be covered by `web/uitest` at little cost.
- **History in the browser** (`history.js`) and **findings in the browser**
  (`findings.js`) have no tests.
- **Exports**: nothing checks that `floating` and `requested` reach the JSON
  and GraphML output, or that the `ui` parameter of the HTML export is
  validated.
- **Analysis**: nothing checks `Node.Bytes`, `.gitignore` through
  `git ls-files`, a manifest change between cached runs, the minified-file
  rule, `jsconfig.json`, or the poetry, pdm and Pipfile lock files.
- **Command line**: single-dash long flags, errors printed once, errors on
  stderr, `-v`, `--no-history`, and the loopback default address are not
  tested.
- **The M4 acceptance runs** (ripgrep, gson, Serilog, Pester) and the
  performance targets REQ-LANG-030 and REQ-PERF-001 have no benchmark or
  scripted end-to-end run. REQ-LANG-029 has `BenchmarkColdAnalysis`, which
  `go test` does not run by default.
- **Tests that skip**: the gopls reference test needs `gopls`, and the
  extension's panel and server tests need `DEPPHUNTER` to name a built binary.
