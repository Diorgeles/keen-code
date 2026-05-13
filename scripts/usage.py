#!/usr/bin/env python3
"""Fetch OpenCode usage data and save it as CSV."""

import argparse
import csv
import datetime as dt
import os
import re
import sys
import time
from typing import Optional

import requests

SERVER_URL = "https://opencode.ai/_server"
DEFAULT_SERVER_ID = "bfd684bfc2e4eed05cd0b518f5e4eafd3f3376e3938abb9e536e7c03df831e5c"
DEFAULT_INSTANCE = "server-fn:11"
COST_DIVISOR = 100_000_000
DEFAULT_OUTPUT = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "bench", "usage.csv")
DEFAULT_SINCE = "2026-05-13T19:00:00"
ULID_ALPHABET = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"


def fetch_page(workspace: str, token: str, page: int, server_id: str) -> str:
    body = {
        "t": {
            "t": 9, "i": 0, "l": 2,
            "a": [
                {"t": 1, "s": workspace},
                {"t": 0, "s": page},
            ],
            "o": 0,
        },
        "f": 31,
        "m": [],
    }
    resp = requests.post(
        SERVER_URL,
        json=body,
        cookies={"auth": token, "oc_locale": "en"},
        timeout=(5, 10),
        headers={
            "accept": "*/*",
            "content-type": "application/json",
            "connection": "close",
            "origin": "https://opencode.ai",
            "referer": f"https://opencode.ai/workspace/{workspace}/usage",
            "user-agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36",
            "x-server-id": server_id,
            "x-server-instance": DEFAULT_INSTANCE,
        },
    )
    resp.raise_for_status()
    return resp.text


def string_field(text: str, name: str) -> Optional[str]:
    key = rf'(?:{name}|"{name}"|\\"{name}\\")'
    m = re.search(rf'{key}\s*:\s*(?:"|\\")([^"\\]+)(?:"|\\")', text)
    return m.group(1) if m else None


def int_field(text: str, name: str) -> Optional[int]:
    key = rf'(?:{name}|"{name}"|\\"{name}\\")'
    m = re.search(rf'{key}\s*:\s*(\d+|null)', text)
    if not m or m.group(1) == "null":
        return None
    return int(m.group(1))


def usage_timestamp_ms(usage_id: str) -> Optional[int]:
    value = usage_id.removeprefix("usg_")[:10]
    if len(value) != 10:
        return None
    timestamp = 0
    for char in value:
        try:
            digit = ULID_ALPHABET.index(char.upper())
        except ValueError:
            return None
        timestamp = timestamp * 32 + digit
    return timestamp


def cutoff_ms(since: str) -> int:
    try:
        cutoff = dt.datetime.fromisoformat(since)
    except ValueError as err:
        raise ValueError("--since must use ISO format, for example 2026-05-13T19:00:00") from err
    if cutoff.tzinfo is None:
        cutoff = cutoff.replace(tzinfo=dt.datetime.now().astimezone().tzinfo)
    return int(cutoff.timestamp() * 1000)


def parse_records(text: str) -> list[dict]:
    records = []
    ids = list(re.finditer(r'usg_[A-Za-z0-9._-]+', text))
    for i, id_m in enumerate(ids):
        end = ids[i + 1].start() if i + 1 < len(ids) else len(text)
        chunk = text[id_m.start():end]

        model = string_field(chunk, "model")
        input_tokens = int_field(chunk, "inputTokens")
        output_tokens = int_field(chunk, "outputTokens")
        cost = int_field(chunk, "cost")
        if model is None or input_tokens is None or output_tokens is None or cost is None:
            continue

        records.append({
            "id": id_m.group(0),
            "timestamp_ms": usage_timestamp_ms(id_m.group(0)),
            "model": model,
            "input_tokens": input_tokens,
            "output_tokens": output_tokens,
            "reasoning_tokens": int_field(chunk, "reasoningTokens") or 0,
            "cache_read_tokens": int_field(chunk, "cacheReadTokens") or 0,
            "cost": cost,
            "session_id": string_field(chunk, "sessionID"),
        })
    return records


def fetch_all_records(workspace: str, token: str, server_id: str, since_ms: int, max_pages: int = 50) -> list[dict]:
    all_records = []
    page = 1
    while page <= max_pages:
        print(f"Fetching page {page}...", file=sys.stderr, end=" ", flush=True)
        try:
            text = fetch_page(workspace, token, page, server_id)
        except requests.exceptions.Timeout:
            print("timeout, stopping.", file=sys.stderr)
            break
        records = parse_records(text)
        old_records = [r for r in records if r["timestamp_ms"] is not None and r["timestamp_ms"] < since_ms]
        records = [r for r in records if r["timestamp_ms"] is None or r["timestamp_ms"] >= since_ms]
        print(f"{len(records)} records", file=sys.stderr, flush=True)
        if not records:
            break
        all_records.extend(records)
        if old_records:
            print("Cutoff reached, stopping.", file=sys.stderr)
            break
        page += 1
        time.sleep(2)

    print(f"Total: {len(all_records)} records across {page - 1} pages.", file=sys.stderr)
    return all_records


def write_csv(path: str, records: list[dict]) -> None:
    fieldnames = [
        "id",
        "session_id",
        "model",
        "input_tokens",
        "output_tokens",
        "reasoning_tokens",
        "cache_read_tokens",
        "cost",
        "cost_usd",
    ]
    directory = os.path.dirname(path)
    if directory:
        os.makedirs(directory, exist_ok=True)
    with open(path, "w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()
        for record in records:
            row = dict(record)
            row.pop("timestamp_ms", None)
            row["session_id"] = row["session_id"] or ""
            row["cost_usd"] = row["cost"] / COST_DIVISOR
            writer.writerow(row)


def count_sessions(records: list[dict]) -> int:
    sessions = set()
    for r in records:
        if r["session_id"]:
            sessions.add(r["session_id"])
    return len(sessions)


def main():
    parser = argparse.ArgumentParser(description="Download OpenCode usage records as CSV")
    parser.add_argument(
        "--workspace", "-w",
        default=os.environ.get("OPENCODE_WORKSPACE", "wrk_01KQWY2QZAA77SKQ17MV4KZZPJ"),
        help="Workspace ID (default: env OPENCODE_WORKSPACE)",
    )
    parser.add_argument(
        "--token", "-t",
        default=os.environ.get("OPENCODE_TOKEN"),
        help="Auth token (default: env OPENCODE_TOKEN)",
    )
    parser.add_argument(
        "--server-id",
        default=os.environ.get("OPENCODE_SERVER_ID", DEFAULT_SERVER_ID),
        help="x-server-id header (update if requests start 404ing after a deployment)",
    )
    parser.add_argument(
        "--max-pages",
        type=int,
        default=50,
        help="Maximum number of usage pages to fetch (default: 50)",
    )
    parser.add_argument(
        "--output", "-o",
        default=DEFAULT_OUTPUT,
        help="CSV output path (default: bench/usage.csv)",
    )
    parser.add_argument(
        "--since",
        default=DEFAULT_SINCE,
        help="Only fetch records at or after this ISO timestamp (default: 2026-05-13T19:00:00 local time)",
    )
    args = parser.parse_args()

    if not args.token:
        print("Error: set OPENCODE_TOKEN env var or pass --token", file=sys.stderr)
        sys.exit(1)

    if args.max_pages < 1:
        print("Error: --max-pages must be positive", file=sys.stderr)
        sys.exit(1)

    try:
        since_ms = cutoff_ms(args.since)
    except ValueError as err:
        print(f"Error: {err}", file=sys.stderr)
        sys.exit(1)

    all_records = fetch_all_records(args.workspace, args.token, args.server_id, since_ms, args.max_pages)
    if not all_records:
        print("No usage records found.", file=sys.stderr)
        sys.exit(1)

    try:
        write_csv(args.output, all_records)
    except OSError as err:
        print(f"Error: failed to write CSV: {err}", file=sys.stderr)
        sys.exit(1)

    print(f"Wrote {len(all_records)} records across {count_sessions(all_records)} sessions to {args.output}")


if __name__ == "__main__":
    main()
