#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/.venv/bin/activate"

cleanup() {
  rm -rf _docs mkdocs.yml site
}
trap cleanup EXIT

pip show mkdocs-material &>/dev/null || pip install mkdocs-material

mkdir -p _docs
cp README.md TOUR.md ROADMAP.md CHANGELOG.md CONTRIBUTING.md AGENTS.md _docs/
cp -r docs _docs/
cp -r .ai-interactions _docs/ai-interactions
cp -r assets _docs/
cp .github/pages/extra.css _docs/
sed -i '' 's/<div align="center">/<div align="center" markdown="1">/g' _docs/README.md
sed -i '' 's/\.ai-interactions\//ai-interactions\//g' _docs/README.md _docs/TOUR.md

python3 .github/pages/gen_nav.py _docs > mkdocs.yml

mkdocs serve
