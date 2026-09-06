# taxonomy/ontology — Vendored vocabulary directory

This directory is the home for vendored external ontology content and
author-contributed hierarchies consumed by the Gibson security graph.

## What belongs here

| File pattern | Content | Status |
|---|---|---|
| `*.ttl` | Raw Turtle triples from external vocabularies | **Deferred** — see below |
| `*.yaml` | YAML-format ontologies authored in Gibson syntax | Example files present |
| `alignments.yaml` | Cross-vocabulary equivalence declarations | Example file present |
| `THIRD-PARTY-LICENSES.md` | Per-file license attribution | Placeholder present |

### Vocabularies planned for vendoring (tracked in issue #28)

- **MITRE ATT&CK** — enterprise technique hierarchy in Turtle (`mitre-attack.ttl`).
  License: CC BY 4.0. Vendoring is owner work tracked in the follow-up milestone.
- **MITRE ATLAS** — AI/ML threat techniques (`mitre-atlas.ttl`).
  License: CC BY 4.0. Same milestone.
- **CWE subset** — weakness entries relevant to the Gibson taxonomy (`cwe-subset.ttl`).
  License: public domain / MITRE terms. Same milestone.
- **SOC2 controls** — authored hierarchy, not a third-party file (`soc2-controls.yaml`).
- **NIST AI RMF** — authored hierarchy (`nist-ai-rmf.yaml`).

This PR ships the structure and the loader (`taxonomy.EmbeddedOntology()`), not
the content. The actual Turtle files will be added in the follow-up once
licensing review is complete.

## File format

Ontology YAML files use the format defined in
`taxonomy/ontology_schema.go`. See `alignments.yaml.example` and
`soc2-controls.yaml.example` for worked examples.

## How the daemon loads this directory

The daemon calls `taxonomy.EmbeddedOntology()` at startup to receive an
`fs.FS` rooted at this directory. It skips any `.example` files and walks the
remaining `.yaml` files, parsing each with `taxonomy.Parse`. Raw `.ttl` files
are passed through to the `Reasoner` as `OntologyExtension.RawTriples` for
future ingestion.

## Adding a new vocabulary

1. Place the Turtle or YAML file in this directory.
2. Add a license attribution entry to `THIRD-PARTY-LICENSES.md`.
3. Run `make test` in `opensource/sdk/` — the embed picks up the new file
   automatically.
4. Open a PR using the `feat:` prefix with the new file and the license entry.
