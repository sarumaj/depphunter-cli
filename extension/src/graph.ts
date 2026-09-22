// The graph document, as internal/graph declares it.
//
// Generated from internal/graph/graph.go - do not edit. To change it, change the Go
// declaration and run:
//
//	go test ./internal/graph -update
//
// The check that this file still says what Go says runs with the ordinary tests.

export interface Graph {
  root: string;
  generatedAt: string;
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export interface GraphNode {
  id: string;
  kind: 'dir' | 'file' | 'symbol' | 'ecosystem' | 'package';
  name: string;
  path?: string;
  parent?: string;
  lang?: string;
  loc?: number;
  symbolKind?: string;
  line?: number;
  version?: string;

  /**
   * Requested is the specifier a manifest asked for when a lock file pinned it to
   * another version, e.g. "^4.2.0" for version 4.3.1.
   */
  requested?: string;

  /**
   * Floating marks an external package that is not fixed to one version: it will
   * resolve to something else once it is installed again.
   */
  floating?: boolean;

  /**
   * Transitive marks a package no file in the project imports: it is on the map
   * because something the project depends on depends on it.
   */
  transitive?: boolean;

  /**
   * Index is the package index or mirror the package resolves from, and
   * IndexUnknown marks one that only the repository's own configuration names -
   * nothing on this machine vouches for it.
   */
  index?: string;
  indexUnknown?: boolean;

  /**
   * Private marks a package this organization owns (--private, GOPRIVATE). Nothing
   * so marked is named to a public index or sent to the vulnerability database: the
   * request would be the disclosure.
   */
  private?: boolean;

  /**
   * Std marks ecosystems holding a language's standard library, which the UI hides by default.
   */
  std?: boolean;

  /**
   * Unresolved marks packages whose owning module could not be determined from manifests.
   */
  unresolved?: boolean;
}

export interface GraphEdge {
  from: string;
  to: string;
  kind: 'import' | 'reference' | 'depends';
  line?: number;
}
