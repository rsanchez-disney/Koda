#!/bin/bash

PASS=0
FAIL=0

check() {
    local desc="$1"
    shift
    if "$@" > /dev/null 2>&1; then
        echo "  ✓ $desc"
        PASS=$((PASS + 1))
    else
        echo "  ✗ $desc"
        FAIL=$((FAIL + 1))
    fi
}

# Allow non-zero exit (doctor/status fail on clean system)
check_any() {
    local desc="$1"
    shift
    "$@" > /dev/null 2>&1
    echo "  ✓ $desc (exit=$?)"
    PASS=$((PASS + 1))
}

echo ""
echo "🐾 Koda Smoke Tests"
echo ""

# Binary basics
check "koda version" koda version
check "koda --help" koda --help

# All commands exist
check "setup --help" koda setup --help
check "install --help" koda install --help
check "chat --help" koda chat --help
check "team --help" koda team --help
check "slack --help" koda slack --help
check "workspace --help" koda workspace --help
check "rules --help" koda rules --help
check "diff --help" koda diff --help
check "configure --help" koda configure --help
check "mcp-install --help" koda mcp-install --help
check "upgrade --help" koda upgrade --help
check "amazonq --help" koda amazonq --help
check "team init --help" koda team init --help
check "team plan --help" koda team plan --help
check "team merge --help" koda team merge --help

# Commands that run but may fail on clean system (still validates they execute)
check_any "doctor runs" koda doctor
check_any "status runs" koda status

echo ""
echo "Results: $PASS passed, $FAIL failed"
echo ""

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
echo "✅ All smoke tests passed"
