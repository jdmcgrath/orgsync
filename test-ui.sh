#!/bin/bash
# Test script to demonstrate OrgSync UI without creating real repositories

echo "🧪 OrgSync Test Mode Demo"
echo "========================"
echo ""
echo "This script runs OrgSync in test mode to demonstrate the UI"
echo "without creating actual git repositories."
echo ""
echo "Test scenarios:"
echo ""

# Test 1: Small organization
echo "1. Small organization (10 repos, low failure rate):"
echo "   go run . --test --test-repos=10 --test-fail-rate=0.05 test-small-org"
echo ""

# Test 2: Medium organization  
echo "2. Medium organization (30 repos, normal failure rate):"
echo "   go run . --test --test-repos=30 --test-fail-rate=0.1 test-medium-org"
echo ""

# Test 3: Large organization
echo "3. Large organization (100 repos, higher failure rate):"
echo "   go run . --test --test-repos=100 --test-fail-rate=0.15 test-large-org"
echo ""

# Test 4: Stress test
echo "4. Stress test (200 repos, mixed failures):"
echo "   go run . --test --test-repos=200 --test-fail-rate=0.2 test-stress"
echo ""

echo "Choose a test scenario (1-4) or press Enter to run the default test:"
read -r choice

case $choice in
    1)
        echo "Running small organization test..."
        go run . --test --test-repos=10 --test-fail-rate=0.05 test-small-org
        ;;
    2)
        echo "Running medium organization test..."
        go run . --test --test-repos=30 --test-fail-rate=0.1 test-medium-org
        ;;
    3)
        echo "Running large organization test..."
        go run . --test --test-repos=100 --test-fail-rate=0.15 test-large-org
        ;;
    4)
        echo "Running stress test..."
        go run . --test --test-repos=200 --test-fail-rate=0.2 test-stress
        ;;
    *)
        echo "Running default test (20 repos, 10% failure rate)..."
        go run . --test test-org
        ;;
esac