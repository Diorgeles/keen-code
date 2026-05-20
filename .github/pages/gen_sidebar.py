#!/usr/bin/env python3
"""Generate _sidebar.md for Docsify from the keen-code site directory."""

import os
import re
import sys

ACRONYMS = {"rfc", "prd", "ui", "repl", "llm", "cli", "mcp", "btw", "api", "oauth", "ai"}


def format_name(raw: str) -> str:
    name = re.sub(r"\.md$", "", raw)
    name = re.sub(r"^(?:output|prompt)-\d+[_-]", "", name)
    name = name.replace("-", " ").replace("_", " ")
    words = [w.upper() if w.lower() in ACRONYMS else w.capitalize() for w in name.split()]
    return " ".join(words)


def format_dir(raw: str) -> str:
    m = re.match(r"^(phase|issue)-(\d+)$", raw)
    if m:
        return f"{m.group(1).capitalize()} {m.group(2)}"
    return format_name(raw)


def list_ai_interactions(base: str) -> list[str]:
    lines = []
    ai_dir = os.path.join(base, ".ai-interactions")
    if not os.path.isdir(ai_dir):
        return lines

    lines.append("\n- **AI Interactions**")

    for section in ["outputs", "prompts", "tasks"]:
        section_dir = os.path.join(ai_dir, section)
        if not os.path.isdir(section_dir):
            continue
        lines.append(f"  - **{section.capitalize()}**")

        for subdir in sorted(os.listdir(section_dir), key=lambda d: (re.sub(r"\d+", "", d), int(m.group()) if (m := re.search(r"\d+", d)) else 0)):
            subdir_path = os.path.join(section_dir, subdir)
            if not os.path.isdir(subdir_path):
                continue
            lines.append(f"    - **{format_dir(subdir)}**")

            for fname in sorted(os.listdir(subdir_path), key=lambda f: int(m.group()) if (m := re.search(r"\d+", f)) else 0):
                if not fname.endswith(".md"):
                    continue
                rel = os.path.join(".ai-interactions", section, subdir, fname)
                display = format_name(fname)
                lines.append(f"      - [{display}]({rel})")

    return lines


def list_docs(base: str) -> list[str]:
    docs_dir = os.path.join(base, "docs")
    if not os.path.isdir(docs_dir):
        return []
    lines = ["\n- **Documentation**"]
    for fname in sorted(os.listdir(docs_dir)):
        if not fname.endswith(".md"):
            continue
        display = format_name(fname)
        lines.append(f"  - [{display}](docs/{fname})")
    return lines


def main():
    base = sys.argv[1] if len(sys.argv) > 1 else "."

    print("- [Home](/)")
    for title, path in [
        ("Tour", "TOUR.md"),
        ("Roadmap", "ROADMAP.md"),
        ("Changelog", "CHANGELOG.md"),
        ("Contributing", "CONTRIBUTING.md"),
        ("Agents", "AGENTS.md"),
    ]:
        if os.path.isfile(os.path.join(base, path)):
            print(f"- [{title}]({path})")

    for line in list_docs(base):
        print(line)

    for line in list_ai_interactions(base):
        print(line)


if __name__ == "__main__":
    main()
