#!/usr/bin/env python3
"""Generate mkdocs.yml for MkDocs Material from the keen-code staging directory."""

import os
import re
import sys
import yaml

ACRONYMS = {"rfc", "prd", "ui", "repl", "llm", "cli", "mcp", "btw", "api", "oauth", "ai"}

BASE_CONFIG = {
    "site_name": "Keen Code",
    "site_url": "https://mochow13.github.io/keen-code",
    "repo_url": "https://github.com/mochow13/keen-code",
    "repo_name": "mochow13/keen-code",
    "theme": {
        "name": "material",
        "logo": "assets/keen-code.png",
        "favicon": "assets/keen-code.png",
        "palette": [
            {
                "media": "(prefers-color-scheme)",
                "toggle": {
                    "icon": "material/brightness-auto",
                    "name": "Switch to light mode",
                },
            },
            {
                "media": "(prefers-color-scheme: light)",
                "scheme": "default",
                "primary": "black",
                "accent": "indigo",
                "toggle": {
                    "icon": "material/brightness-7",
                    "name": "Switch to dark mode",
                },
            },
            {
                "media": "(prefers-color-scheme: dark)",
                "scheme": "slate",
                "primary": "black",
                "accent": "indigo",
                "toggle": {
                    "icon": "material/brightness-4",
                    "name": "Switch to system preference",
                },
            },
        ],
        "features": [
            "navigation.sections",
            "navigation.top",
            "search.highlight",
            "search.suggest",
            "content.code.copy",
        ],
        "font": {"code": "Cascadia Code"},
    },
    "extra_css": ["extra.css"],
    "markdown_extensions": [
        "pymdownx.highlight",
        "pymdownx.superfences",
        "tables",
        "admonition",
        "md_in_html",
        {"toc": {"permalink": True}},
    ],
}


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


def num_key(name: str) -> int:
    m = re.search(r"\d+", name)
    return int(m.group()) if m else 0


def build_docs_nav(base: str) -> list:
    docs_dir = os.path.join(base, "docs")
    if not os.path.isdir(docs_dir):
        return []
    entries = []
    for fname in sorted(os.listdir(docs_dir)):
        if fname.endswith(".md"):
            entries.append({format_name(fname): f"docs/{fname}"})
    return entries


def build_ai_interactions_nav(base: str) -> list:
    for dirname in ("ai-interactions", ".ai-interactions"):
        ai_dir = os.path.join(base, dirname)
        if os.path.isdir(ai_dir):
            nav_prefix = dirname
            break
    else:
        return []

    sections = []
    for section in ["outputs", "prompts", "tasks"]:
        section_dir = os.path.join(ai_dir, section)
        if not os.path.isdir(section_dir):
            continue

        subsections = []
        for subdir in sorted(os.listdir(section_dir), key=lambda d: (re.sub(r"\d+", "", d), num_key(d))):
            subdir_path = os.path.join(section_dir, subdir)
            if not os.path.isdir(subdir_path):
                continue

            files = []
            for fname in sorted(os.listdir(subdir_path), key=num_key):
                if fname.endswith(".md"):
                    rel = f"{nav_prefix}/{section}/{subdir}/{fname}"
                    files.append({format_name(fname): rel})

            if files:
                subsections.append({format_dir(subdir): files})

        if subsections:
            sections.append({section.capitalize(): subsections})

    return sections


def build_nav(base: str) -> list:
    nav = [{"Home": "README.md"}]
    for title, path in [
        ("Tour", "TOUR.md"),
        ("Roadmap", "ROADMAP.md"),
        ("Changelog", "CHANGELOG.md"),
        ("Contributing", "CONTRIBUTING.md"),
        ("Agents", "AGENTS.md"),
    ]:
        if os.path.isfile(os.path.join(base, path)):
            nav.append({title: path})

    docs = build_docs_nav(base)
    if docs:
        nav.append({"Documentation": docs})

    ai = build_ai_interactions_nav(base)
    if ai:
        nav.append({"AI Interactions": ai})

    return nav


def main():
    base = sys.argv[1] if len(sys.argv) > 1 else "."
    config = dict(BASE_CONFIG)
    config["docs_dir"] = base
    config["nav"] = build_nav(base)
    print(yaml.dump(config, default_flow_style=False, sort_keys=False, allow_unicode=True))


if __name__ == "__main__":
    main()
