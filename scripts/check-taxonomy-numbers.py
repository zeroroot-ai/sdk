#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Zero Root AI
"""Fail when a CoreNodeType or CoreRelationType number changes meaning.

taxonomy/core.yaml declares each enum value (sdk#132), so reordering the file is
free and a renumber takes a deliberate edit. Two hazards remain, and neither is
visible in the tree alone:

  a live number is changed      the value means something else from now on
  a RETIRED entry is deleted    its number becomes free, and the next type added
                                takes it, which changes the meaning of every
                                value already recorded

The proto cannot catch either, because it is generated: edit core.yaml, run
`make generate`, and both sides agree on the new, wrong answer. The only ledger
of what a number used to mean is history, so this compares the base revision of
core.yaml with the head one.

Usage:
    check-taxonomy-numbers.py                 # base from git, head from the tree
    check-taxonomy-numbers.py BASE HEAD       # two files, for the selftest
    check-taxonomy-numbers.py --selftest
"""
from __future__ import annotations

import subprocess
import sys

import yaml

CORE_YAML = "taxonomy/core.yaml"
SECTIONS = ("node_types", "relationship_types")


def entries(doc: dict, section: str) -> dict[str, dict]:
    """Map name -> {number, retired} for one section."""
    out = {}
    for item in doc.get(section) or []:
        name = item.get("name")
        if name is None:
            continue
        out[name] = {
            "number": item.get("number"),
            "retired": bool(item.get("retired")),
        }
    return out


def compare(base_doc: dict, head_doc: dict) -> list[str]:
    """Return every problem, so one run reports all of them."""
    problems: list[str] = []
    for section in SECTIONS:
        base = entries(base_doc, section)
        head = entries(head_doc, section)

        for name, was in base.items():
            now = head.get(name)
            if now is None:
                if was["retired"]:
                    problems.append(
                        f"{section}: {name} was retired holding number {was['number']} "
                        f"and has been deleted. That frees {was['number']} for the next "
                        f"entry, which would change the meaning of every {was['number']} "
                        f"already recorded. A retired entry stays in the file forever: "
                        f"that is what reserves its number."
                    )
                # Deleting a LIVE entry is a separate decision and no longer
                # renumbers anything, so it is not this guard's business.
                continue
            if was["number"] is None:
                # The base revision predates declared numbers: its values came
                # from position. sdk#132 added them, and the generated proto was
                # byte-identical afterwards, which is the proof that the declared
                # values equal the position-derived ones. There is nothing to
                # compare, and comparing would fail every entry once.
                continue
            if was["number"] != now["number"]:
                problems.append(
                    f"{section}: {name} was {was['number']} and is now {now['number']}. "
                    f"An enum value is a wire contract: data recorded as "
                    f"{was['number']} would be read as something else. Add a new entry "
                    f"with the next free number instead."
                )
            if was["retired"] and not now["retired"]:
                problems.append(
                    f"{section}: {name} was retired and is live again at number "
                    f"{now['number']}. Un-retiring reuses a value that was taken out "
                    f"of service. Add a new entry instead."
                )

        # A new entry must not take a number the base already used, retired
        # included. The parser rejects a duplicate inside one file; this catches
        # reuse of a number whose entry was deleted in the same change.
        used = {v["number"]: n for n, v in base.items() if v["number"] is not None}
        for name, now in head.items():
            if name in base or now["number"] is None:
                continue
            if now["number"] in used:
                problems.append(
                    f"{section}: new entry {name} takes number {now['number']}, which "
                    f"belonged to {used[now['number']]}. Pick a number no entry has "
                    f"ever held."
                )
    return problems


def git_base(ref: str) -> dict:
    """core.yaml as of ref. An empty tree at ref means nothing to compare."""
    try:
        raw = subprocess.run(
            ["git", "show", f"{ref}:{CORE_YAML}"],
            check=True, capture_output=True, text=True,
        ).stdout
    except subprocess.CalledProcessError as exc:
        print(
            f"could not read {CORE_YAML} at {ref}: {exc.stderr.strip()}\n"
            f"This guard compares against history and cannot run without it. "
            f"A shallow checkout needs fetch-depth: 0.",
            file=sys.stderr,
        )
        sys.exit(2)
    return yaml.safe_load(raw) or {}


def load(path: str) -> dict:
    with open(path, encoding="utf-8") as fh:
        return yaml.safe_load(fh) or {}


def report(problems: list[str]) -> int:
    if not problems:
        print("✅ every taxonomy enum number means what it did on the base revision")
        return 0
    print(f"❌ {len(problems)} taxonomy enum number problem(s):\n", file=sys.stderr)
    for p in problems:
        print(f"  - {p}\n", file=sys.stderr)
    return 1


def selftest() -> int:
    base = {
        "node_types": [
            {"name": "technique", "number": 16},
            {"name": "compliance_signal", "number": 17, "retired": True},
            {"name": "scope", "number": 18},
        ],
        "relationship_types": [{"name": "USED_TOOL", "number": 1}],
    }

    def head(nodes, rels=None):
        return {
            "node_types": nodes,
            "relationship_types": rels or [{"name": "USED_TOOL", "number": 1}],
        }

    cases = [
        (
            "unchanged passes",
            head(base["node_types"]),
            None,
        ),
        (
            "deleting a retired entry fails",
            head([
                {"name": "technique", "number": 16},
                {"name": "scope", "number": 18},
            ]),
            "frees 17",
        ),
        (
            "changing a live number fails",
            head([
                {"name": "technique", "number": 16},
                {"name": "compliance_signal", "number": 17, "retired": True},
                {"name": "scope", "number": 99},
            ]),
            "was 18 and is now 99",
        ),
        (
            "un-retiring fails",
            head([
                {"name": "technique", "number": 16},
                {"name": "compliance_signal", "number": 17},
                {"name": "scope", "number": 18},
            ]),
            "is live again",
        ),
        (
            "reusing a deleted entry's number fails",
            head([
                {"name": "technique", "number": 16},
                {"name": "scope", "number": 18},
                {"name": "new_thing", "number": 17},
            ]),
            "belonged to compliance_signal",
        ),
        (
            "adding at the next free number passes",
            head(base["node_types"] + [{"name": "new_thing", "number": 21}]),
            None,
        ),
        (
            "reordering passes",
            head(list(reversed(base["node_types"]))),
            None,
        ),
        (
            "a renumbered relationship fails",
            head(base["node_types"], [{"name": "USED_TOOL", "number": 7}]),
            "was 1 and is now 7",
        ),
    ]

    # The migration case needs a base with no numbers at all, which is what
    # origin/main looked like before sdk#132.
    unnumbered = {
        "node_types": [{"name": n["name"]} for n in base["node_types"]],
        "relationship_types": [{"name": "USED_TOOL"}],
    }
    migration = compare(unnumbered, head(base["node_types"]))
    if migration:
        print(f"SELFTEST FAIL: a base with no numbers reported {migration}")
        return 1
    print("selftest ok: a base with no numbers is the one-time migration")

    failures = 0
    for name, head_doc, want in cases:
        problems = compare(base, head_doc)
        if want is None:
            if problems:
                print(f"SELFTEST FAIL: {name}: {problems}")
                failures += 1
            else:
                print(f"selftest ok: {name}")
            continue
        if not problems:
            print(f"SELFTEST FAIL: {name}: accepted, expected a failure")
            failures += 1
        elif not any(want in p for p in problems):
            print(f"SELFTEST FAIL: {name}: no message contains {want!r}: {problems}")
            failures += 1
        else:
            print(f"selftest ok: {name}")

    if failures:
        print(f"SELFTEST FAILED ({failures})")
        return 1
    print("SELFTEST PASSED")
    return 0


def main(argv: list[str]) -> int:
    if argv and argv[0] == "--selftest":
        return selftest()
    if len(argv) == 2:
        return report(compare(load(argv[0]), load(argv[1])))
    if argv:
        print(__doc__, file=sys.stderr)
        return 2
    ref = "origin/main"
    return report(compare(git_base(ref), load(CORE_YAML)))


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
