#!/bin/bash

# AutoUI Platform Development Stop Script

set -e

echo "🛑 Stopping AutoUI Platform Development Environment..."

# Function to stop a service by PID file
stop_service() {
    local service_name=$1
    local pid_file=$2
    
    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if kill -0 "$pid" 2>/dev/null; then
            echo "🔪 Stopping $service_name (PID: $pid)..."
            kill -TERM "$pid"
            
            # Wait for graceful shutdown
            local count=0
            while kill -0 "$pid" 2>/dev/null && [ $count -lt 10 ]; do
                sleep 1
                count=$((count + 1))
            done
            
            # Force kill if still running
            if kill -0 "$pid" 2>/dev/null; then
                echo "⚠️  Force killing $service_name..."
                kill -KILL "$pid"
            fi
            
            echo "✅ $service_name stopped"
        else
            echo "ℹ️  $service_name was not running"
        fi
        rm -f "$pid_file"
    else
        echo "ℹ️  No PID file found for $service_name"
    fi
}

# Stop backend service
stop_service "backend" ".backend.pid"

# Stop frontend service
stop_service "frontend" ".frontend.pid"

# Also kill any remaining node/go processes (be careful with this)
echo "🧹 Cleaning up any remaining processes..."

# Kill any remaining go processes for this project
pkill -f "go run cmd/main.go" 2>/dev/null || true

# Kill any remaining npm start processes
pkill -f "npm start" 2>/dev/null || true

echo ""
echo "✅ All services have been stopped"
echo ""
echo "📝 Log files are preserved in the logs/ directory"
echo "🔄 To restart: ./scripts/start-dev.sh"