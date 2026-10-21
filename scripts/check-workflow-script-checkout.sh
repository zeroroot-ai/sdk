#!/usr/bin/env bash
# Fail when a workflow job runs a script of this repo before it checks the repo
# out.
#
#   check-workflow-script-checkout.sh [workflow-dir]   (default .github/workflows)
#
# A job that runs `bash scripts/...` or `./scripts/...` needs this repo in its
# workspace at that step. The script fails with "No such file or directory"
# (exit 127) at run time, and a job that only runs on a dispatch can carry the
# defect for a long time. The v0.209.0 fan-out failed this way twice: resolve-tag
# had no checkout, and the fan-out job ran the script after a checkout of the
# consumer repo.
#
#  - A checkout with no `repository:` (or this repo) and no `path:` puts this
#    repo in the workspace.
#  - A checkout of another repo into the default path replaces the workspace,
#    so this repo is gone again.
#  - A checkout with a `path:` other than the workspace itself ("." or
#    github.workspace) changes neither.
#  - After a sparse checkout, the sparse patterns must cover the script. In cone
#    mode (the actions/checkout default) a pattern names a directory: it covers
#    each file under it, each file directly in one of its parents, and each file
#    at the root. With cone mode off, a pattern is a glob, and a pattern that
#    ends in / covers each file under it.
#
#  - A sparse checkout of this repo into the default path stays sparse for a
#    later checkout of this repo into the default path, because
#    actions/checkout reuses the .git there. A later checkout whose repository
#    is an expression can be this repo, so it fails too.
#
# A job that uses a reusable workflow has no steps and is skipped.
set -euo pipefail

dir="${1:-.github/workflows}"

python3 - "$dir" <<'PY'
import fnmatch
import pathlib
import re
import sys

import yaml

# `bash scripts/x.sh`, `sh ./scripts/x.sh` and `./scripts/x.sh`.
SCRIPT_CALL = re.compile(r'(?:(?:^|[\s;&|(])(?:bash|sh)\s+(?:\./)?|(?:^|[\s;&|(])\./)(scripts/[^\s;&|)"\']+)', re.M)
DEFAULT_PATHS = {"", ".", "./", "${{ github.workspace }}"}
THIS_REPOS = {"", "zeroroot-ai/sdk", "${{ github.repository }}"}
failures = []


def checkout_effect(step):
    """Return ("this", sparse), ("other", None) or None for a checkout step.

    A checkout of another repo into the default path replaces the workspace, so
    this repo is gone from it. A checkout into a path changes neither.
    """
    with_ = step.get("with") or {}
    # "." and the workspace itself are the default path.
    if str(with_.get("path", "")).strip() not in DEFAULT_PATHS:
        return None
    if str(with_.get("repository", "")).strip() not in THIS_REPOS:
        return ("other", None)
    sparse = [line.strip() for line in str(with_.get("sparse-checkout", "")).splitlines() if line.strip()]
    cone = str(with_.get("sparse-checkout-cone-mode", "true")).strip().lower() != "false"
    return ("this", (sparse, cone))


def sparse_covers(script, sparse, cone):
    """Return True when the sparse checkout puts script in the workspace."""
    parent = script.rpartition("/")[0]
    for pattern in sparse:
        if cone:
            directory = pattern.strip("/")
            # Under the directory, or directly in the directory or in one of its parents.
            if script.startswith(directory + "/") or parent == "" or directory == parent or directory.startswith(parent + "/"):
                return True
        else:
            glob = pattern.lstrip("/")
            if fnmatch.fnmatch(script, glob) or (glob.endswith("/") and script.startswith(glob)):
                return True
    return False


for path in sorted(pathlib.Path(sys.argv[1]).glob("*.y*ml")):
    doc = yaml.safe_load(path.read_text())
    for job_id, job in ((doc or {}).get("jobs") or {}).items():
        present = False
        sparse, cone = [], True
        sparse_left = False
        for index, step in enumerate((job or {}).get("steps") or []):
            name = step.get("name", f"step {index + 1}")
            if str(step.get("uses", "")).startswith("actions/checkout@"):
                with_ = step.get("with") or {}
                in_default = str(with_.get("path", "")).strip() in DEFAULT_PATHS
                repo = str(with_.get("repository", "")).strip()
                if sparse_left and in_default and (repo in THIS_REPOS or "${{" in repo):
                    failures.append(f"{path}: job {job_id}, step '{name}' checks out into a workspace that still holds a sparse checkout of this repo")
                effect = checkout_effect(step)
                if effect is not None:
                    # A checkout of another repo replaces the .git, so no sparse state is left.
                    sparse_left = effect[0] == "this" and bool(effect[1][0])
                if effect is not None:
                    present = effect[0] == "this"
                    sparse, cone = effect[1] or ([], True)
                continue
            run = step.get("run")
            if not run:
                continue
            for script in SCRIPT_CALL.findall(run):
                if not present:
                    failures.append(f"{path}: job {job_id}, step '{name}' runs {script}, and this repo is not in the workspace")
                elif sparse and not sparse_covers(script, sparse, cone):
                    failures.append(f"{path}: job {job_id}, step '{name}' runs {script}, and the sparse checkout does not cover it")

if failures:
    for line in failures:
        print(f"::error::{line}")
    sys.exit(1)
print("every job that runs a script of the repo has the repo in its workspace first")
PY
