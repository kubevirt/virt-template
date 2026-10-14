#!/usr/bin/env python3
#
# This file is part of the KubeVirt project
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Copyright The KubeVirt Authors.
#
"""Report Go directive changes between a PR merge base and head commit."""

from __future__ import annotations

import html
import re
import subprocess
import sys
from pathlib import PurePosixPath


DIRECTIVES = ("go", "toolchain")
DIRECTIVE_PATTERN = re.compile(r"^(go|toolchain)\s+(\S+)")


def git_output(*args: str) -> bytes:
    return subprocess.check_output(["git", *args])


def read_directives(revision: str, path: str) -> dict[str, str]:
    result = subprocess.run(
        ["git", "show", f"{revision}:{path}"],
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        return {}

    directives: dict[str, str] = {}
    for raw_line in result.stdout.splitlines():
        match = DIRECTIVE_PATTERN.match(raw_line.strip())
        if match:
            name, version = match.groups()
            if name == "toolchain" and version.startswith("go"):
                version = version.removeprefix("go")
            directives[name] = version
    return directives


def code_cell(value: str) -> str:
    return f"<code>{html.escape(value).replace('|', '&#124;')}</code>"


def main() -> int:
    if len(sys.argv) != 3:
        print(f"Usage: {sys.argv[0]} BASE_SHA HEAD_SHA", file=sys.stderr)
        return 2

    base_sha, head_sha = sys.argv[1:]
    for revision in (base_sha, head_sha):
        git_output("rev-parse", "--verify", f"{revision}^{{commit}}")

    merge_base = git_output("merge-base", base_sha, head_sha).decode().strip()
    changed_paths = git_output(
        "diff", "--name-only", "--no-renames", "-z", merge_base, head_sha
    ).decode().split("\0")
    module_paths = sorted(
        path
        for path in changed_paths
        if PurePosixPath(path).name in {"go.mod", "go.work"}
        and "vendor" not in PurePosixPath(path).parts
    )

    changes: list[tuple[str, str, str, str]] = []
    for path in module_paths:
        base_directives = read_directives(merge_base, path)
        head_directives = read_directives(head_sha, path)
        for name in DIRECTIVES:
            before = base_directives.get(name)
            after = head_directives.get(name)
            if before != after:
                changes.append((path, name, before or "not set", after or "not set"))

    if not changes:
        return 0

    print("Renovate changed Go version directives in this PR:")
    print()
    print("| Module or workspace | Directive | Base | PR |")
    print("| --- | --- | --- | --- |")
    for path, name, before, after in changes:
        print(
            f"| {code_cell(path)} | {code_cell(name)} | "
            f"{code_cell(before)} | {code_cell(after)} |"
        )
    print()
    print("Verify that build and release environments support the new Go version before merging.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
