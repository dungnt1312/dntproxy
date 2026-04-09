#!/bin/bash
# Dev environment for dntproxy
# Runs Go backend (with auto-rebuild) + Vite UI dev server

set -e

PORT=${PORT:-20199}

echo "🔨 Building Go backend..."
go build -o dntproxy.exe ./cmd/dntproxy/

echo "🚀 Starting backend on port $PORT..."
./dntproxy.exe --port="$PORT" &
BACKEND_PID=$!

echo "🎨 Starting UI dev server..."
cd ui
bun i
bun run build
UI_PID=$!
cd ..

# Trap Ctrl+C to kill both processes
cleanup() {
  echo ""
  echo "🛑 Shutting down..."
  kill $BACKEND_PID 2>/dev/null
  kill $UI_PID 2>/dev/null
  exit 0
}
trap cleanup SIGINT SIGTERM

echo ""
echo "✅ Dev environment ready!"
echo "   Backend:  http://localhost:$PORT"
echo "   UI:       http://localhost:5173"
echo ""
echo "Press Ctrl+C to stop all services."

# Wait for either process to exit
wait
