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
        content = f.read()

    # Find JSON object starting with {"JobId"
    match = re.search(r'(\{"JobId".*?\})(?:\s*\d+|$)', content, re.DOTALL)
    if not match:
        print(f"❌ Could not find JSON in {scan_file}")
        return None

    # Clean control characters
    json_str = match.group(1)
    json_str = ''.join(c for c in json_str if ord(c) >= 32 or c in '\n\r\t')

    try:
        data = json.loads(json_str)
        return data
    except json.JSONDecodeError as e:
        print(f"❌ Failed to parse JSON: {e}")
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
