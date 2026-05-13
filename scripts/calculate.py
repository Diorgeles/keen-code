#!/usr/bin/env python3
"""Calculate usage totals for session IDs from an exported OpenCode usage CSV."""

import argparse
import csv
import sys

COST_DIVISOR = 100_000_000


def normalize_session_id(session_id: str) -> str:
    return session_id.replace("-", "")[:30]


def parse_session_ids(value: str) -> list[str]:
    return [session_id for session_id in (item.strip() for item in value.split(",")) if session_id]


def int_field(row: dict, name: str) -> int:
    value = row.get(name, "")
    if value == "":
        return 0
    return int(float(value))


def cost_usd(row: dict) -> float:
    raw = row.get("cost", "")
    if raw != "":
        return int(float(raw)) / COST_DIVISOR

    usd = row.get("cost_usd", "")
    if usd != "":
        return float(usd)

    return 0.0


def load_rows(path: str) -> list[dict]:
    with open(path, newline="") as f:
        return list(csv.DictReader(f))


def calculate(rows: list[dict], session_ids: list[str]) -> list[dict]:
    results = []
    for session_id in session_ids:
        key = normalize_session_id(session_id)
        matches = [
            row for row in rows
            if normalize_session_id(row.get("session_id", "")) == key
        ]
        models = sorted({row.get("model", "") for row in matches if row.get("model", "")})
        results.append({
            "session_id": session_id,
            "match_key": key,
            "requests": len(matches),
            "cost_usd": sum(cost_usd(row) for row in matches),
            "input_tokens": sum(int_field(row, "input_tokens") for row in matches),
            "output_tokens": sum(int_field(row, "output_tokens") for row in matches),
            "reasoning_tokens": sum(int_field(row, "reasoning_tokens") for row in matches),
            "cache_read_tokens": sum(int_field(row, "cache_read_tokens") for row in matches),
            "models": ", ".join(models),
        })
    return results


def print_results(results: list[dict]) -> None:
    print(
        f"{'Session':<38} {'Requests':>8} {'Cost':>10} "
        f"{'Input':>10} {'Output':>10} {'Reasoning':>10} {'Cache Read':>12}  Models"
    )
    print("-" * 128)
    for result in results:
        print(
            f"{result['session_id']:<38} "
            f"{result['requests']:>8} "
            f"${result['cost_usd']:>9.4f} "
            f"{result['input_tokens']:>10} "
            f"{result['output_tokens']:>10} "
            f"{result['reasoning_tokens']:>10} "
            f"{result['cache_read_tokens']:>12}  "
            f"{result['models']}"
        )

    total_requests = sum(result["requests"] for result in results)
    total_cost = sum(result["cost_usd"] for result in results)
    total_input = sum(result["input_tokens"] for result in results)
    total_output = sum(result["output_tokens"] for result in results)
    total_reasoning = sum(result["reasoning_tokens"] for result in results)
    total_cache = sum(result["cache_read_tokens"] for result in results)

    print("-" * 128)
    print(
        f"{'TOTAL':<38} "
        f"{total_requests:>8} "
        f"${total_cost:>9.4f} "
        f"{total_input:>10} "
        f"{total_output:>10} "
        f"{total_reasoning:>10} "
        f"{total_cache:>12}"
    )


def main() -> None:
    parser = argparse.ArgumentParser(description="Calculate usage totals for session IDs from CSV")
    parser.add_argument(
        "--sessions",
        "-s",
        required=True,
        help="Comma-separated session IDs. Dashes are removed and the first 30 characters are matched.",
    )
    parser.add_argument(
        "--csv",
        "-c",
        default="usage.csv",
        help="Usage CSV file exported by scripts/usage.py (default: usage.csv)",
    )
    args = parser.parse_args()

    session_ids = parse_session_ids(args.sessions)
    if not session_ids:
        print("Error: --sessions must include at least one non-empty session ID", file=sys.stderr)
        sys.exit(1)

    try:
        rows = load_rows(args.csv)
    except OSError as err:
        print(f"Error: failed to read CSV: {err}", file=sys.stderr)
        sys.exit(1)

    print_results(calculate(rows, session_ids))


if __name__ == "__main__":
    main()
