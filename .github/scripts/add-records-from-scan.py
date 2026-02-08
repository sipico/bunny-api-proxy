#!/usr/bin/env python3
"""
Add DNS records from scan results to a bunny.net zone.

Usage:
    python3 add-records-from-scan.py <domain> <zone_id> <scan_file> <api_key> <api_url> [max_records]

Args:
    domain: Domain name (for logging)
    zone_id: Zone ID to add records to
    scan_file: Path to scan result log file
    api_key: Bunny.net API key
    api_url: Bunny.net API URL (e.g., https://api.bunny.net)
    max_records: Maximum number of records to add (default: 5)
"""

import json
import re
import subprocess
import sys


def extract_scan_results(scan_file):
    """Extract JSON scan results from curl log file."""
    with open(scan_file, 'r') as f:
        lines = f.readlines()

    # Find the line with JSON (starts with {" and doesn't have curl markers)
    json_line = None
    for line in lines:
        # Skip curl verbose output lines
        if line.startswith(('*', '>', '<', '{', '}')):
            continue
        # Look for line that looks like JSON
        if line.strip().startswith('{"JobId"'):
            json_line = line
            break

    # If not found, try the last line (curl sometimes puts JSON at the end)
    if not json_line:
        for line in reversed(lines):
            stripped = line.strip()
            if stripped.startswith('{"JobId"'):
                json_line = stripped
                break

    if not json_line:
        print(f"❌ Could not find JSON in {scan_file}")
        return None

    # Extract just the JSON object by counting braces
    start_idx = json_line.find('{"JobId"')
    if start_idx == -1:
        print(f"❌ Could not find JSON start in line")
        return None

    json_line = json_line[start_idx:]

    # Find the matching closing brace
    brace_count = 0
    end_idx = 0
    for i, char in enumerate(json_line):
        if char == '{':
            brace_count += 1
        elif char == '}':
            brace_count -= 1
            if brace_count == 0:
                end_idx = i + 1
                break

    if end_idx == 0:
        print(f"❌ Could not find JSON end")
        return None

    json_str = json_line[:end_idx]

    try:
        data = json.loads(json_str)
        return data
    except json.JSONDecodeError as e:
        print(f"❌ Failed to parse JSON: {e}")
        print(f"   JSON length: {len(json_str)} chars")
        return None


def add_record(zone_id, record, api_key, api_url):
    """Add a single record to the zone via API."""
    # Convert scan record to AddRecordRequest format
    add_req = {
        "Type": record["Type"],
        "Name": record["Name"],
        "Value": record["Value"],
        "Ttl": record["Ttl"],
        "Priority": record.get("Priority") or 0,
        "Weight": record.get("Weight") or 0,
        "Port": record.get("Port") or 0,
        "Flags": 0,
        "Tag": "",
        "Disabled": False,
        "Comment": "Added from DNS scan"
    }

    # Call API via curl
    result = subprocess.run([
        'curl', '-s', '-X', 'PUT',
        '-H', f'AccessKey: {api_key}',
        '-H', 'Content-Type: application/json',
        '-d', json.dumps(add_req),
        f'{api_url}/dnszone/{zone_id}/records'
    ], capture_output=True, text=True)

    if result.returncode != 0:
        return None, f"curl failed: {result.stderr[:100]}"

    try:
        resp = json.loads(result.stdout)
        return resp, None
    except json.JSONDecodeError:
        return None, f"Invalid JSON response: {result.stdout[:100]}"


def main():
    if len(sys.argv) < 6:
        print("Usage: add-records-from-scan.py <domain> <zone_id> <scan_file> <api_key> <api_url> [max_records]")
        sys.exit(1)

    domain = sys.argv[1]
    zone_id = sys.argv[2]
    scan_file = sys.argv[3]
    api_key = sys.argv[4]
    api_url = sys.argv[5]
    max_records = int(sys.argv[6]) if len(sys.argv) > 6 else 5

    print(f"Adding records for {domain} (Zone ID: {zone_id})")
    print(f"Reading scan results from: {scan_file}")
    print(f"Max records to add: {max_records}")
    print()

    # Extract scan results
    data = extract_scan_results(scan_file)
    if not data:
        sys.exit(1)

    records = data.get('Records', [])
    if not records:
        print("⚠️  No records found in scan results")
        sys.exit(0)

    print(f"Found {len(records)} records in scan")
    print(f"Adding first {min(max_records, len(records))} records...")
    print()

    # Add records
    success_count = 0
    error_count = 0

    type_names = {0: 'A', 1: 'AAAA', 2: 'CNAME', 3: 'TXT', 4: 'MX', 8: 'SRV'}

    for i, record in enumerate(records[:max_records]):
        type_name = type_names.get(record['Type'], f"Type{record['Type']}")
        name = record.get('Name', '@')
        value = record['Value'][:40] + '...' if len(record['Value']) > 40 else record['Value']

        print(f"  {i+1}. {type_name:6s} {name:20s} -> {value}")

        resp, error = add_record(zone_id, record, api_key, api_url)

        if error:
            print(f"     ❌ Failed: {error}")
            error_count += 1
        elif resp and resp.get('Id'):
            print(f"     ✅ Created record ID: {resp['Id']}")
            success_count += 1
        else:
            print(f"     ⚠️  Unexpected response: {resp}")
            error_count += 1

    print()
    print(f"Summary: {success_count} added, {error_count} failed")

    if error_count > 0:
        sys.exit(1)


if __name__ == '__main__':
    main()
