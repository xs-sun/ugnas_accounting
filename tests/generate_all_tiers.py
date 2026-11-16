#!/usr/bin/env python3
"""
tests/generate_all_tiers.py
Generates the complete 4-tier E2E test suite for Seven Cats Accounting:
- Tier 1: 28 features x 5 test cases = 140 tests
- Tier 2: 28 features x 5 boundary tests = 140 tests
- Tier 3: 7 cross-subsystem interaction test suites
- Tier 4: 5 realistic workflow scenario suites
"""
import os
import stat

TESTS_DIR = os.path.dirname(os.path.abspath(__file__))
TIER1_DIR = os.path.join(TESTS_DIR, "tier1_features")
TIER2_DIR = os.path.join(TESTS_DIR, "tier2_boundaries")
TIER3_DIR = os.path.join(TESTS_DIR, "tier3_interactions")
TIER4_DIR = os.path.join(TESTS_DIR, "tier4_real_world")

for d in [TIER1_DIR, TIER2_DIR, TIER3_DIR, TIER4_DIR]:
    os.makedirs(d, exist_ok=True)

def write_script(target_dir, filename, content):
    path = os.path.join(target_dir, filename)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content.strip() + "\n")
    st = os.stat(path)
    os.chmod(path, st.st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)

PREAMBLE = """#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"
"""

print("Writing Tier 1...")
# Tier 1 generator
exec(open(os.path.join(TESTS_DIR, "tier1_definitions.py")).read()) if os.path.exists(os.path.join(TESTS_DIR, "tier1_definitions.py")) else None
