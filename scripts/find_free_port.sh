#!/usr/bin/env bash

# Usage: ./scripts/find_free_port.sh [start_port]
# Outputs the first available (free) TCP port starting from start_port (default: 8080)

START_PORT="${1:-8080}"
PORT=$START_PORT

is_port_in_use() {
    local check_port=$1
    if command -v nc &> /dev/null; then
        nc -z 127.0.0.1 "$check_port" 2>/dev/null
    elif command -v lsof &> /dev/null; then
        lsof -i :"$check_port" &>/dev/null
    elif command -v ss &> /dev/null; then
        ss -lnt "( sport = :$check_port )" | grep -q ":$check_port"
    else
        (echo > "/dev/tcp/127.0.0.1/$check_port") 2>/dev/null
    fi
}

while is_port_in_use "$PORT"; do
    PORT=$((PORT + 1))
done

echo "$PORT"
