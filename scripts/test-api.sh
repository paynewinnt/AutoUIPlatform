#!/bin/bash

# AutoUI Platform API Test Script

set -e

API_BASE_URL="http://localhost:8080/api/v1"

echo "🧪 Testing AutoUI Platform API..."
echo ""

# Function to test an endpoint
test_endpoint() {
    local method=$1
    local endpoint=$2
    local description=$3
    local expected_code=${4:-200}
    
    echo "Testing: $description"
    echo "  $method $API_BASE_URL$endpoint"
    
    local response_code
    if [ "$method" = "GET" ]; then
        response_code=$(curl -s -o /dev/null -w "%{http_code}" "$API_BASE_URL$endpoint")
    else
        response_code=$(curl -s -o /dev/null -w "%{http_code}" -X "$method" "$API_BASE_URL$endpoint")
    fi
    
    if [ "$response_code" -eq "$expected_code" ]; then
        echo "  ✅ Success (HTTP $response_code)"
    else
        echo "  ❌ Failed (HTTP $response_code, expected $expected_code)"
    fi
    echo ""
}

# Test health check
test_endpoint "GET" "/health" "Health Check"

# Test auth endpoints (these should return 400 for bad request, not 200)
test_endpoint "POST" "/auth/login" "Login Endpoint" 400
test_endpoint "POST" "/auth/register" "Register Endpoint" 400

# Test protected endpoints (these should return 401 for unauthorized)
test_endpoint "GET" "/users/profile" "Get Profile (Protected)" 401
test_endpoint "GET" "/projects" "Get Projects (Protected)" 401
test_endpoint "GET" "/environments" "Get Environments (Protected)" 401
test_endpoint "GET" "/devices" "Get Devices (Protected)" 401

echo "🎯 API Test Summary:"
echo "   - Health check should return 200"
echo "   - Auth endpoints should return 400 (bad request)"
echo "   - Protected endpoints should return 401 (unauthorized)"
echo ""
echo "🔧 To test with authentication:"
echo "   1. Register a user: curl -X POST $API_BASE_URL/auth/register -H 'Content-Type: application/json' -d '{\"username\":\"test\",\"email\":\"test@example.com\",\"password\":\"123456\"}'"
echo "   2. Login: curl -X POST $API_BASE_URL/auth/login -H 'Content-Type: application/json' -d '{\"username\":\"test\",\"password\":\"123456\"}'"
echo "   3. Use the returned token in Authorization header: -H 'Authorization: Bearer <token>'"