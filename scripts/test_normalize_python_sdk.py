#!/usr/bin/env python3
"""Focused tests for generated Python SDK whitespace normalization."""

from __future__ import annotations

import importlib.util
from pathlib import Path
import tempfile
import unittest


SCRIPT_PATH = Path(__file__).with_name("normalize-python-sdk.py")


def load_normalizer():
    spec = importlib.util.spec_from_file_location("normalize_python_sdk", SCRIPT_PATH)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"unable to load {SCRIPT_PATH}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class NormalizePythonSDKTests(unittest.TestCase):
    def test_normalizes_python_files_recursively_and_ignores_other_files(self) -> None:
        normalizer = load_normalizer()

        with tempfile.TemporaryDirectory() as temporary_directory:
            sdk_root = Path(temporary_directory)
            nested_root = sdk_root / "nested"
            nested_root.mkdir()
            python_file = sdk_root / "resource.py"
            stub_file = nested_root / "resource.pyi"
            text_file = nested_root / "notes.txt"
            python_file.write_bytes(b"publication surface\n\n\n")
            stub_file.write_bytes(b"typed surface\n\n")
            text_file.write_bytes(b"leave this alone\n\n\n")

            normalizer.normalize_python_sdk(sdk_root)

            self.assertEqual(python_file.read_bytes(), b"publication surface\n")
            self.assertEqual(stub_file.read_bytes(), b"typed surface\n")
            self.assertEqual(text_file.read_bytes(), b"leave this alone\n\n\n")


if __name__ == "__main__":
    unittest.main()
