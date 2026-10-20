#!/usr/bin/env bash
# Fail when a workflow job runs a script of this repo before it checks the repo
# out.
#
#   check-workflow-script-checkout.sh [workflow-dir]   (default .github/workflows)
#
# A job that runs `bash scripts/...` or `./scripts/...` needs the repo in its
# workspace. A job with no earlier actions/checkout step fails with "No such
# file or directory" (exit 127) at run time, and a job that only runs on a
# dispatch can carry the defect for a long time. The v0.209.0 fan-out failed
# this way in resolve-tag. A job that uses a reusable workflow has no steps and
# is skipped.
set -euo pipefail

dir="${1:-.github/workflows}"

python3 - "$dir" <<'PY'
import pathlib
import re
import sys

import yaml

SCRIPT_CALL = re.compile(r'(^|[\s;&|(])(bash|sh)\s+(\./)?scripts/|(^|[\s;&|(])\./scripts/', re.M)
failures = []

for path in sorted(pathlib.Path(sys.argv[1]).glob("*.y*ml")):
    doc = yaml.safe_load(path.read_text())
    for job_id, job in ((doc or {}).get("jobs") or {}).items():
        checked_out = False
        for index, step in enumerate((job or {}).get("steps") or []):
            if str(step.get("uses", "")).startswith("actions/checkout@"):
                checked_out = True
            run = step.get("run")
            if run and SCRIPT_CALL.search(run) and not checked_out:
                name = step.get("name", f"step {index + 1}")
                failures.append(f"{path}: job {job_id}, step '{name}' runs a script of the repo before any checkout")

if failures:
    for line in failures:
        print(f"::error::{line}")
    sys.exit(1)
print("every job that runs a script of the repo checks the repo out first")
PY
