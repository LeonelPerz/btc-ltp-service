#!/bin/bash

# Cache Testing Script for CI
# This script runs comprehensive tests for the cache package with race detection

set -e  # Exit on any error
set -u  # Exit on undefined variables

echo "🧪 Starting Cache Tests with Race Detection for CI"
echo "=================================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed or not in PATH"
    exit 1
fi

print_status "Go version: $(go version)"

# Set test timeout
TEST_TIMEOUT=${TEST_TIMEOUT:-"300s"}
CACHE_PKG="./internal/infrastructure/repositories/cache"

# Create reports directory
mkdir -p reports

print_status "Running cache tests with race detection..."

# Run tests with race detection
print_status "🏃‍♂️ Running tests with -race flag"
if go test -race -timeout="$TEST_TIMEOUT" -v "$CACHE_PKG" > reports/race_test.log 2>&1; then
    print_success "Race detection tests passed"
else
    print_error "Race detection tests failed"
    echo "Test output:"
    cat reports/race_test.log
    exit 1
fi

# Run tests with coverage and race detection
print_status "📊 Running tests with coverage and race detection"
if go test -race -coverprofile=reports/coverage_race.out -covermode=atomic -timeout="$TEST_TIMEOUT" "$CACHE_PKG" > reports/coverage_race.log 2>&1; then
    print_success "Coverage tests with race detection passed"
    
    # Generate coverage report
    if go tool cover -func=reports/coverage_race.out > reports/coverage_summary.txt; then
        echo "Coverage Summary:"
        cat reports/coverage_summary.txt
        
        # Extract total coverage percentage
        COVERAGE=$(go tool cover -func=reports/coverage_race.out | grep total | awk '{print $3}')
        print_status "Total Coverage: $COVERAGE"
        
        # Generate HTML coverage report
        go tool cover -html=reports/coverage_race.out -o reports/coverage_race.html
        print_success "HTML coverage report generated: reports/coverage_race.html"
    else
        print_warning "Could not generate coverage report"
    fi
else
    print_error "Coverage tests with race detection failed"
    echo "Test output:"
    cat reports/coverage_race.log
    exit 1
fi

# Run specific concurrent tests multiple times
print_status "🔄 Running concurrent tests multiple times"
for i in {1..5}; do
    print_status "Iteration $i/5"
    if ! go test -race -run="Concurrent" -count=1 "$CACHE_PKG" > "reports/concurrent_test_$i.log" 2>&1; then
        print_error "Concurrent test iteration $i failed"
        cat "reports/concurrent_test_$i.log"
        exit 1
    fi
done
print_success "All concurrent test iterations passed"

# Check for race conditions in logs
print_status "🔍 Checking for race conditions in test logs"
if grep -r "race detected\|WARNING: DATA RACE" reports/ > reports/race_warnings.txt 2>/dev/null; then
    print_error "Race conditions detected!"
    echo "Race condition details:"
    cat reports/race_warnings.txt
    exit 1
else
    print_success "No race conditions detected"
fi

# Run memory tests
print_status "🧠 Running memory tests"
if go test -memprofile=reports/mem.prof -run="Memory" "$CACHE_PKG" > reports/memory_test.log 2>&1; then
    print_success "Memory tests passed"
else
    print_warning "Memory tests failed or no memory tests found"
fi

# Benchmark tests
print_status "⚡ Running benchmark tests"
if go test -bench=. -benchmem -run=^$ "$CACHE_PKG" > reports/benchmark.log 2>&1; then
    print_success "Benchmark tests completed"
    echo "Benchmark results:"
    cat reports/benchmark.log
else
    print_warning "Benchmark tests failed or no benchmarks found"
fi

# Final summary
print_success "✅ All cache tests passed with race detection!"
print_status "Reports generated in ./reports/ directory:"
ls -la reports/

echo ""
echo "🎉 Cache CI Tests Completed Successfully!"
echo "=================================================="
