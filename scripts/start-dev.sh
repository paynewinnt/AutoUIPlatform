#!/bin/bash

# AutoUI Platform Development Startup Script

set -e

echo "🚀 Starting AutoUI Platform Development Environment..."

# Check if we're in the right directory
if [ ! -f "go.mod" ]; then
    echo "❌ Error: go.mod not found. Please run this script from the project root directory."
    exit 1
fi

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check dependencies
echo "🔍 Checking dependencies..."

if ! command_exists go; then
    echo "❌ Go is not installed or not in PATH"
    exit 1
fi

if ! command_exists node; then
    echo "❌ Node.js is not installed or not in PATH"
    exit 1
fi

if ! command_exists npm; then
    echo "❌ npm is not installed or not in PATH"
    exit 1
fi

echo "✅ All dependencies are available"

# Create necessary directories
echo "📁 Creating necessary directories..."
mkdir -p uploads screenshots logs

# Setup environment variables
if [ ! -f .env ]; then
    echo "📝 Creating .env file..."
    cp .env.example .env
    echo "⚠️  Please edit .env file to configure your database connection"
fi

# Start services in the background
echo "🔧 Starting backend service..."
cd backend
go mod tidy
nohup go run cmd/main.go > ../logs/backend.log 2>&1 &
BACKEND_PID=$!
cd ..

echo "🎨 Starting frontend service..."
cd frontend
if [ ! -d "node_modules" ]; then
    echo "📦 Installing frontend dependencies..."
    npm install
fi
nohup npm start > ../logs/frontend.log 2>&1 &
FRONTEND_PID=$!
cd ..

# Save PIDs for later cleanup
echo $BACKEND_PID > .backend.pid
echo $FRONTEND_PID > .frontend.pid

echo ""
echo "🎉 AutoUI Platform is starting up!"
echo ""
echo "📊 Service Information:"
echo "   Backend PID: $BACKEND_PID"
echo "   Frontend PID: $FRONTEND_PID"
echo ""
echo "🌐 URLs (will be available in a few moments):"
echo "   Frontend: http://localhost:3000"
echo "   Backend API: http://localhost:8080/api/v1"
echo "   Health Check: http://localhost:8080/api/v1/health"
echo ""
echo "📋 Logs:"
echo "   Backend: tail -f logs/backend.log"
echo "   Frontend: tail -f logs/frontend.log"
echo ""
echo "🛑 To stop services:"
echo "   ./scripts/stop-dev.sh"
echo ""

# Wait a bit and check if services are running
sleep 5

echo "🔍 Checking service status..."

if kill -0 $BACKEND_PID 2>/dev/null; then
    echo "✅ Backend service is running (PID: $BACKEND_PID)"
else
    echo "❌ Backend service failed to start"
    echo "Check logs: tail logs/backend.log"
fi

if kill -0 $FRONTEND_PID 2>/dev/null; then
    echo "✅ Frontend service is running (PID: $FRONTEND_PID)"
else
    echo "❌ Frontend service failed to start"
    echo "Check logs: tail logs/frontend.log"
fi

echo ""
echo "💡 Tip: Use 'tail -f logs/backend.log logs/frontend.log' to monitor both services"