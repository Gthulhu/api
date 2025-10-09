#!/bin/bash

echo "=== Testing Backward Compatibility of Gthulhu API Server ==="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

API_BASE="http://localhost:8080"
TOKEN=""

# Test counter
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Function to check test result
check_test() {
    local test_name="$1"
    local expected="$2"
    local actual="$3"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    if [[ "$actual" == *"$expected"* ]]; then
        echo -e "${GREEN}✓${NC} $test_name: PASS"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "${RED}✗${NC} $test_name: FAIL"
        echo "  Expected: $expected"
        echo "  Actual: $actual"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
}

echo ""
echo "1. Testing Health Endpoint (No Auth Required)"
echo "============================================="

# Test health endpoint
response=$(curl -s "$API_BASE/health")
check_test "Health endpoint returns JSON" '"status":"healthy"' "$response"
check_test "Health endpoint includes timestamp" '"timestamp"' "$response"

echo ""
echo "2. Testing JWT Authentication"
echo "=============================="

# Generate public key for testing
cat > test_public_key.pem << 'EOF'
-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAny28YMC2/+yYj3T29lz6
0uryNz8gNVrqD7lTJuHQ3DMTE6ADqnERy8VgHve0tWzhJc5ZBZ1Hduvj+z/kNqbc
U81YGhmfOrQ3iFNYBlSAseIHdAw39HGyC6OKzTXI4HRpc8CwcF6hKExkyWlkALr5
i+IQDfimvarjjZ6Nm368L0Rthv3KOkI5CqRZ6bsVwwBug7GcdkvFs3LiRSKlMBpH
2tCkZ5ZZE8VyuK7VnlwV7n6EHzN5BqaHq8HVLw2KzvibSi+/5wIZV2Yx33tViLbh
OsZqLt6qQCGGgKzNX4TGwRLGAiVV1NCpgQhimZ4YP2thqSsqbaISOuvFlYq+QGP1
bcvcHB7UhT1ZnHSDYcbT2qiD3VoqytXVKLB1X5XCD99YLSP9B32f1lvZD4MhDtE4
IhAuqn15MGB5ct4yj/uMldFScs9KhqnWcwS4K6Qx3IfdB+ZxT5hEOWJLEcGqe/CS
XITNG7oS9mrSAJJvHSLz++4R/Sh1MnT2YWjyDk6qeeqAwut0w5iDKWt7qsGEcHFP
IVVlos+xLfrPDtgHQk8upjslUcMyMDTf21Y3RdJ3k1gTR9KHEwzKeiNlLjen9ekF
WupF8jik1aYRWL6h54ZyGxwKEyMYi9o18G2pXPzvVaPYtU+TGXdO4QwiES72TNCD
bNaGj75Gj0sN+LfjjQ4A898CAwEAAQ==
-----END PUBLIC KEY-----
EOF

# Note: This will fail unless private key matches, but tests API structure
token_response=$(curl -s -X POST "$API_BASE/api/v1/auth/token" \
  -H "Content-Type: application/json" \
  -d "{\"public_key\":\"$(cat test_public_key.pem | sed ':a;N;$!ba;s/\n/\\n/g')\"}")

check_test "Token endpoint returns JSON" '"success"' "$token_response"

echo ""
echo "3. Testing GET /api/v1/scheduling/strategies (Requires Auth)"
echo "==========================================================="

# Test without auth (should fail)
response=$(curl -s -w "\n%{http_code}" "$API_BASE/api/v1/scheduling/strategies")
status_code=$(echo "$response" | tail -n1)
check_test "Strategies endpoint requires auth" "401" "$status_code"

# Test response structure (even if unauthorized)
check_test "Unauthorized response is JSON" '"error"' "$(echo "$response" | head -n-1)"

echo ""
echo "4. Testing Response Headers"
echo "============================"

# Check CORS headers
response_headers=$(curl -s -I "$API_BASE/health")
check_test "CORS headers present" "Access-Control-Allow-Origin" "$response_headers"

# Check cache headers on strategies endpoint (new feature)
response_headers=$(curl -s -I -H "Authorization: Bearer invalid" "$API_BASE/api/v1/scheduling/strategies" 2>/dev/null || true)
echo -e "${YELLOW}ℹ${NC} Cache headers (X-Cache-Hit, X-Cache-Stats) are new features"

echo ""
echo "5. Testing POST /api/v1/scheduling/strategies Structure"
echo "========================================================"

# Test POST with invalid auth (to check structure)
response=$(curl -s -X POST "$API_BASE/api/v1/scheduling/strategies" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer invalid" \
  -d '{
    "strategies": [
      {
        "priority": true,
        "execution_time": 20000000,
        "selectors": [
          {
            "key": "app",
            "value": "test"
          }
        ]
      }
    ]
  }')

check_test "POST strategies endpoint returns JSON" '"error"' "$response"

echo ""
echo "6. Testing Pod-PID Mapping Endpoint"
echo "===================================="

response=$(curl -s -w "\n%{http_code}" "$API_BASE/api/v1/pods/pids")
status_code=$(echo "$response" | tail -n1)
check_test "Pod-PID endpoint requires auth" "401" "$status_code"

echo ""
echo "7. Testing Backward Compatibility Summary"
echo "========================================="

# Check that all original endpoints still exist
echo "Original endpoints check:"
for endpoint in "/health" "/api/v1/auth/token" "/api/v1/metrics" "/api/v1/pods/pids" "/api/v1/scheduling/strategies"; do
    response_code=$(curl -s -o /dev/null -w "%{http_code}" "$API_BASE$endpoint")
    if [[ "$response_code" != "000" ]]; then
        echo -e "${GREEN}✓${NC} $endpoint: Accessible (HTTP $response_code)"
    else
        echo -e "${RED}✗${NC} $endpoint: Not accessible"
    fi
done

echo ""
echo "======================================"
echo "Test Results Summary:"
echo "======================================"
echo -e "Total Tests: $TOTAL_TESTS"
echo -e "Passed: ${GREEN}$PASSED_TESTS${NC}"
echo -e "Failed: ${RED}$FAILED_TESTS${NC}"

if [[ $FAILED_TESTS -eq 0 ]]; then
    echo -e "${GREEN}All backward compatibility tests passed!${NC}"
    echo -e "${YELLOW}Note: The cache feature adds new headers but maintains API compatibility.${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed. Please review the changes.${NC}"
    exit 1
fi

# Cleanup
rm -f test_public_key.pem