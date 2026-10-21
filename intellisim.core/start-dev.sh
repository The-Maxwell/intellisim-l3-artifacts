#!/bin/bash

echo "Starting IntelliSim Development Environment..."
echo ""

# Check if MongoDB is running
echo "Checking MongoDB..."
if ! pgrep -x "mongod" > /dev/null; then
    echo "MongoDB is not running. Please start MongoDB first."
    echo "Run: sudo systemctl start mongod"
    echo "or: brew services start mongodb-community"
    exit 1
fi
echo "MongoDB is running."
echo ""

# Function to cleanup on exit
cleanup() {
    echo ""
    echo "Stopping servers..."
    kill $BACKEND_PID $FRONTEND_PID 2>/dev/null
    echo "Servers stopped."
    exit 0
}

trap cleanup EXIT INT TERM

# Start backend
echo "Starting Backend Server..."
go run ./cmd/intelli-sim &
BACKEND_PID=$!

# Wait for backend to start
sleep 5

# Start frontend
echo "Starting Frontend Server..."
cd ui && npm run dev &
FRONTEND_PID=$!
cd ..

echo ""
echo "========================================"
echo "IntelliSim is running..."
echo "========================================"
echo "Backend: http://localhost:8080"
echo "Frontend: http://localhost:3000"
echo "========================================"
echo ""
echo "Press Ctrl+C to stop all servers..."

# Wait for user interrupt
wait
