---
id: REQ-PY-016
title: Import roots from PYTHONPATH and tool settings
scope: py
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** add to the import roots of
[REQ-PY-003](REQ-PY-003-python-project-roots.md) the directories Python
projects declare as import or source roots:

- the `PYTHONPATH` of the process running depphunter, split on the system's
  path list separator, relative entries against the repository root;
- the `PYTHONPATH` of each `.env` file (the scan's, and one in the repository
  root and in each project directory read from disk even when git ignores
  it), read with python-dotenv's syntax (`export`, single quotes literal,
  double quotes with escapes, `#` comments, `${NAME}` and `${NAME:-default}`
  from keys set earlier in the file and `${workspaceFolder}` and `${PWD}` as
  the repository root), relative entries against the file's directory;
- in `.vscode/settings.json` (comments and trailing commas allowed) of the
  repository root or a project directory, with `${workspaceFolder}` as that
  directory: `python.analysis.extraPaths`, `python.autoComplete.extraPaths`,
  the `PYTHONPATH` of the `.env` file `python.envFile` names, and the
  `PYTHONPATH` of `terminal.integrated.env.linux`, `.osx` and `.windows`;
- Pyright's and basedpyright's `extraPaths` in `pyrightconfig.json`,
  `basedpyrightconfig.json` (comments allowed) and `[tool.pyright]` /
  `[tool.basedpyright]` of `pyproject.toml`, relative to the file's
  directory; an `executionEnvironments` entry's `root` and `extraPaths`
  **shall** apply only to imports made by files under that root;
- pytest's `pythonpath` in `pytest.ini` or `.pytest.ini` `[pytest]`,
  `tox.ini` `[pytest]`, `setup.cfg` `[tool:pytest]` and `pyproject.toml`
  `[tool.pytest.ini_options]` or `[tool.pytest]`, relative to the file's
  directory;
- mypy's `mypy_path` in `mypy.ini`, `.mypy.ini` and `setup.cfg` `[mypy]` and
  `pyproject.toml` `[tool.mypy]`, split on commas and colons, relative to the
  repository root (mypy's working directory), with `$MYPY_CONFIG_FILE_DIR` as
  the file's directory;
- the source directories of packaging tools, relative to the manifest's
  directory: setuptools `[tool.setuptools] package-dir` (`"" = "src"`, or a
  named package whose directory ends in its own path) and
  `[tool.setuptools.packages.find] where`, `setup.cfg` `[options]
  package_dir` and `[options.packages.find] where`, a literal
  `package_dir={"": ...}` or `find_packages(...)` in `setup.py`, Poetry
  `packages` `from`, Hatch `packages` (their parent) and `sources` of
  `[tool.hatch.build]` and its wheel target, PDM `[tool.pdm.build]
  package-dir` and maturin `python-source`.

Only directories inside the repository that hold Python files **shall**
count: an absolute entry inside the repository **shall** be taken relative to
it, and an entry outside it, in a home directory or naming nothing **shall**
be dropped. Paths written in a file **shall** be split on `;` when there is
one, else on `:` except after a drive letter.

The roots **shall** be tried in this order: a project's own roots (manifest
directories, their `src/` and the packaging source directories) deepest
first; then the configured roots, the process's `PYTHONPATH` first, then those
of files in deeper directories before shallower ones, each file's in the
order it gives them; then the repository root. A directory **shall** be tried
once, in its first place.

Each root a file or the environment adds **shall** be noted for `--explain`
(`import-root`, [REQ-TRC-017](../trc/REQ-TRC-017-resolver-notes.md)) with what
added it and, for an execution environment, the files it serves. Of a `.env`
file only the `PYTHONPATH` key **shall** be kept: no other value **shall**
reach a note, a log, the extraction or the report.

## Rationale

A repository that keeps importable code outside its manifests' directories -
`libs/` on `PYTHONPATH`, a monorepo's shared packages in an editor's extra
paths, tests that import helpers through pytest's `pythonpath` - otherwise
has those imports mapped as undeclared PyPI distributions. The configured
roots stand for paths a run or an editor puts before the working directory,
so they come before the repository root; a sub-project's own modules are more
specific than a path configured for the whole repository, so they come
first, as the deepest roots did before. The process's `PYTHONPATH` is what
actually runs, so it leads the configured roots. `.env` files often hold
secrets, which is why only the one key is read out of them.

## Acceptance criteria

1. Each source above makes an import of a module that only its root reaches
   resolve to the repository file; without the source the import is an
   undeclared distribution.
2. The process's `PYTHONPATH` is taken from the function the plugin is given,
   not read directly, and an entry outside the repository is ignored, as is a
   `.env` entry that climbs out of it.
3. An execution environment's roots do not apply to files outside its root.
4. A sub-project's module shadows a configured root's, the process's
   `PYTHONPATH` shadows a file's, a deeper file's root shadows a shallower
   one's, and a configured root shadows the repository root; the notes name
   each root and what added it.
5. Values of other `.env` keys appear neither in the extraction nor in the
   notes.

## Notes

Pyright's execution environments are matched by the importing file's path
only; a file under an environment's root still also sees the top-level
`extraPaths`, where Pyright would use the environment's alone. Other
configured roots apply to every file, not only to the sub-project that
declares them. Not read: `.pth` files in the repository, `sys.path`
manipulation in code, `conftest.py` `rootdir` insertion (pytest's
`rootdir`/`prepend` import modes), `python.analysis.include`, the per-folder
settings of a `.code-workspace` file, and the process environment's other
variables in `.env` expansion.
