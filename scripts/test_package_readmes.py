#!/usr/bin/env python3
"""Checks that each SDK's package README shows code only in that SDK's language."""

from __future__ import annotations

from pathlib import Path
import re
import unittest


REPO_ROOT = Path(__file__).resolve().parent.parent
PACKAGE_READMES = REPO_ROOT / "docs" / "package-readmes"

# Language -> (committed SDK copy that codegen writes, code fence languages the README may use).
PACKAGES = {
    "nodejs": ("sdk/nodejs/README.md", {"ts", "bash"}),
    "python": ("sdk/python/README.md", {"python", "bash"}),
    "dotnet": ("sdk/dotnet/README.md", {"csharp", "bash"}),
    "go": ("sdk/go/osano/README.md", {"go", "bash"}),
}

FENCE = re.compile(r"^```(\S*)", re.MULTILINE)


def read_text(path: Path) -> str:
    return path.read_bytes().decode("utf-8").replace("\r\n", "\n")


def fence_languages(text: str) -> list[str]:
    # Every other fence closes a block, so only the opening fences name a language.
    return FENCE.findall(text)[::2]


class PackageReadmeTests(unittest.TestCase):
    def test_code_blocks_use_only_the_package_language(self) -> None:
        for language, (_, allowed) in PACKAGES.items():
            with self.subTest(language=language):
                languages = fence_languages(read_text(PACKAGE_READMES / f"{language}.md"))
                self.assertTrue(languages, "README has no code blocks")
                self.assertLessEqual(set(languages), allowed)

    def test_sdk_copies_match_their_sources(self) -> None:
        for language, (copy, _) in PACKAGES.items():
            with self.subTest(language=language):
                self.assertEqual(
                    read_text(REPO_ROOT / copy),
                    read_text(PACKAGE_READMES / f"{language}.md"),
                    f"{copy} is stale; run make codegen or copy docs/package-readmes/{language}.md",
                )

    def test_fence_languages_skips_closing_fences(self) -> None:
        text = "```go\nx\n```\n\n```bash\ny\n```\n"
        self.assertEqual(fence_languages(text), ["go", "bash"])


if __name__ == "__main__":
    unittest.main()
