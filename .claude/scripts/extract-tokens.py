#!/usr/bin/env python3
"""Extract token usage from Claude session transcripts.

Usage:
    ./scripts/extract-tokens.py [transcript_path]
    ./scripts/extract-tokens.py --summary  # Show only summary table

If no path is provided, scans ~/.claude/projects/-home-user-bunny-api-proxy/
"""

import json
import os
import re
import sys
from pathlib import Path
from collections import defaultdict

# Haiku pricing (per 1M tokens)
HAIKU_INPUT_PRICE = 0.80  # $0.80 per 1M input tokens
HAIKU_OUTPUT_PRICE = 4.00  # $4.00 per 1M output tokens
HAIKU_CACHE_READ_PRICE = 0.08  # $0.08 per 1M (90% discount)
HAIKU_CACHE_WRITE_PRICE = 1.00  # $1.00 per 1M (25% premium)

# Opus pricing for comparison
OPUS_INPUT_PRICE = 15.00
OPUS_OUTPUT_PRICE = 75.00


def calculate_cost(tokens: dict, model: str = "haiku") -> float:
    """Calculate cost in USD based on token counts."""
    if model == "haiku":
        input_cost = tokens["input_tokens"] * HAIKU_INPUT_PRICE / 1_000_000
        output_cost = tokens["output_tokens"] * HAIKU_OUTPUT_PRICE / 1_000_000
        cache_read_cost = tokens["cache_read_input_tokens"] * HAIKU_CACHE_READ_PRICE / 1_000_000
        cache_write_cost = tokens["cache_creation_input_tokens"] * HAIKU_CACHE_WRITE_PRICE / 1_000_000
        return input_cost + output_cost + cache_read_cost + cache_write_cost
    else:  # opus
        total_input = (
            tokens["input_tokens"] +
            tokens["cache_creation_input_tokens"] +
            tokens["cache_read_input_tokens"]
        )
        input_cost = total_input * OPUS_INPUT_PRICE / 1_000_000
        output_cost = tokens["output_tokens"] * OPUS_OUTPUT_PRICE / 1_000_000
        return input_cost + output_cost


def extract_issue_number(task: str) -> str:
    """Extract issue number from task description."""
    match = re.search(r'#(\d+)', task)
    return match.group(1) if match else ""


def extract_tokens_from_file(filepath: Path) -> dict:
    """Extract token counts from a JSONL transcript file."""
    totals = {
        "input_tokens": 0,
        "output_tokens": 0,
        "cache_creation_input_tokens": 0,
        "cache_read_input_tokens": 0,
    }

    with open(filepath, 'r') as f:
        for line in f:
            try:
                data = json.loads(line)
                if "message" in data and "usage" in data["message"]:
                    usage = data["message"]["usage"]
                    totals["input_tokens"] += usage.get("input_tokens", 0)
                    totals["output_tokens"] += usage.get("output_tokens", 0)
                    totals["cache_creation_input_tokens"] += usage.get("cache_creation_input_tokens", 0)
                    totals["cache_read_input_tokens"] += usage.get("cache_read_input_tokens", 0)
            except json.JSONDecodeError:
                continue

    return totals


def get_agent_info(filepath: Path) -> dict:
    """Get metadata from first line of transcript."""
    info = {}
    with open(filepath, 'r') as f:
        first_line = f.readline()
        try:
            data = json.loads(first_line)
            info["session_id"] = data.get("sessionId", "unknown")
            info["agent_id"] = data.get("agentId")
            info["slug"] = data.get("slug", "")
            info["git_branch"] = data.get("gitBranch", "")
            # Extract prompt from user message
            if "message" in data and data["message"].get("role") == "user":
                content = data["message"].get("content", "")
                if isinstance(content, str):
                    # Extract first line or issue reference
                    first_line = content.split('\n')[0][:100]
                    info["task"] = first_line
        except json.JSONDecodeError:
            pass
    return info


def format_github_comment(agent_id: str, tokens: dict, info: dict) -> str:
    """Format token usage as GitHub issue comment."""
    total_input = (
        tokens["input_tokens"] +
        tokens["cache_creation_input_tokens"] +
        tokens["cache_read_input_tokens"]
    )

    return f"""## Token Usage (Agent {agent_id})

- **Input tokens:** {total_input:,}
  - Direct: {tokens['input_tokens']:,}
  - Cache creation: {tokens['cache_creation_input_tokens']:,}
  - Cache read: {tokens['cache_read_input_tokens']:,}
- **Output tokens:** {tokens['output_tokens']:,}
- **Total:** {total_input + tokens['output_tokens']:,}

Branch: `{info.get('git_branch', 'unknown')}`"""


def scan_session_dir(session_dir: Path):
    """Scan a session directory for all transcripts."""
    results = []

    # Main session transcript
    session_id = session_dir.name
    main_transcript = session_dir.parent / f"{session_id}.jsonl"
    if main_transcript.exists():
        tokens = extract_tokens_from_file(main_transcript)
        info = get_agent_info(main_transcript)
        results.append({
            "type": "main",
            "path": main_transcript,
            "tokens": tokens,
            "info": info,
        })

    # Subagent transcripts
    subagents_dir = session_dir / "subagents"
    if subagents_dir.exists():
        for agent_file in sorted(subagents_dir.glob("agent-*.jsonl")):
            tokens = extract_tokens_from_file(agent_file)
            info = get_agent_info(agent_file)
            agent_id = agent_file.stem.replace("agent-", "")
            results.append({
                "type": "subagent",
                "agent_id": agent_id,
                "path": agent_file,
                "tokens": tokens,
                "info": info,
            })

    return results


def main():
    # Default project directory
    project_dir = Path.home() / ".claude" / "projects" / "-home-user-bunny-api-proxy"
    summary_only = "--summary" in sys.argv

    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    if args:
        path = Path(args[0])
        if path.suffix == ".jsonl":
            # Single file
            tokens = extract_tokens_from_file(path)
            info = get_agent_info(path)
            agent_id = path.stem.replace("agent-", "") if "agent-" in path.stem else "main"
            print(format_github_comment(agent_id, tokens, info))
            return
        elif path.is_dir():
            project_dir = path

    if not project_dir.exists():
        print(f"Error: Directory not found: {project_dir}")
        sys.exit(1)

    # Find all session directories (those with subagents subdirectory)
    sessions = []
    for item in sorted(project_dir.iterdir()):
        if item.is_dir() and (item / "subagents").exists():
            sessions.append(item)

    if not sessions:
        print("No sessions with subagents found.")
        return

    # Collect all subagent data for summary
    all_subagents = []

    for session_dir in sessions:
        results = scan_session_dir(session_dir)

        if not summary_only:
            print(f"\n{'='*70}")
            print(f"Session: {session_dir.name}")
            print("=" * 70)

        for result in results:
            tokens = result["tokens"]
            info = result["info"]
            total_input = (
                tokens["input_tokens"] +
                tokens["cache_creation_input_tokens"] +
                tokens["cache_read_input_tokens"]
            )
            total = total_input + tokens["output_tokens"]

            if result["type"] == "subagent":
                agent_id = result["agent_id"]
                task = info.get("task", "")
                issue = extract_issue_number(task)
                branch = info.get("git_branch", "")
                haiku_cost = calculate_cost(tokens, "haiku")
                opus_cost = calculate_cost(tokens, "opus")

                all_subagents.append({
                    "agent_id": agent_id,
                    "issue": issue,
                    "branch": branch,
                    "task": task[:50],
                    "tokens": tokens,
                    "total_tokens": total,
                    "haiku_cost": haiku_cost,
                    "opus_cost": opus_cost,
                    "savings": opus_cost - haiku_cost,
                })

                if not summary_only:
                    print(f"\n  Subagent {agent_id}")
                    print(f"    Issue: #{issue}" if issue else "    Issue: N/A")
                    print(f"    Branch: {branch}")
                    print(f"    Task: {task[:60]}...")
                    print(f"    Input:  {total_input:>10,} (direct: {tokens['input_tokens']:,}, cache create: {tokens['cache_creation_input_tokens']:,}, cache read: {tokens['cache_read_input_tokens']:,})")
                    print(f"    Output: {tokens['output_tokens']:>10,}")
                    print(f"    Total:  {total:>10,}")
                    print(f"    Haiku cost: ${haiku_cost:.4f}  (Opus would be: ${opus_cost:.2f})")

    # Print summary table
    print("\n" + "=" * 90)
    print("SUMMARY: SUBAGENT TOKEN USAGE")
    print("=" * 90)
    print(f"{'Issue':<8} {'Agent':<10} {'Output':<10} {'Total':<12} {'Haiku $':<10} {'Opus $':<10} {'Saved':<10}")
    print("-" * 90)

    total_haiku = 0
    total_opus = 0
    for sa in sorted(all_subagents, key=lambda x: x["issue"] or "999"):
        issue = f"#{sa['issue']}" if sa['issue'] else "N/A"
        print(f"{issue:<8} {sa['agent_id']:<10} {sa['tokens']['output_tokens']:<10,} {sa['total_tokens']:<12,} ${sa['haiku_cost']:<9.4f} ${sa['opus_cost']:<9.2f} ${sa['savings']:.2f}")
        total_haiku += sa['haiku_cost']
        total_opus += sa['opus_cost']

    print("-" * 90)
    savings_pct = (1 - total_haiku / total_opus) * 100 if total_opus > 0 else 0
    print(f"{'TOTAL':<8} {'':<10} {'':<10} {'':<12} ${total_haiku:<9.2f} ${total_opus:<9.2f} ${total_opus - total_haiku:.2f} ({savings_pct:.0f}%)")
    print("=" * 90)


if __name__ == "__main__":
    main()
