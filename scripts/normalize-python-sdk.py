#!/usr/bin/env python3
"""Normalize trailing newlines in generated Python SDK source files."""

from __future__ import annotations

import argparse
from pathlib import Path
import re


PYTHON_SUFFIXES = {".py", ".pyi"}
TRAILING_NEWLINES = re.compile(rb"(?:\r\n|\n)+\Z")


def normalize_python_sdk(sdk_root: Path) -> None:
    """Reduce trailing newline runs in generated Python source files to one."""
    for path in sdk_root.rglob("*"):
        if not path.is_file() or path.suffix not in PYTHON_SUFFIXES:
            continue

        original = path.read_bytes()
        newline = b"\r\n" if original.endswith(b"\r\n") else b"\n"
        normalized = TRAILING_NEWLINES.sub(newline, original)
        if normalized != original:
            path.write_bytes(normalized)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("sdk_root", type=Path)
    args = parser.parse_args()
    normalize_python_sdk(args.sdk_root)


if __name__ == "__main__":
    main()
