#!/bin/bash

PID=$1
OUTFILE=$2
CORES=$3

if [ -z "$PID" ] || [ -z "$OUTFILE" ]; then
  echo "Usage: $0 <process-pid> <output-file.csv>"
  exit 1
fi

# Check if process exists
if ! ps -p "$PID" > /dev/null 2>&1; then
  echo "Error: PID $PID not found."
  exit 1
fi

# Header for CSV
echo "timestamp,cpu_percent" > "$OUTFILE"

# Start timestamp counter at -2
TS=-2

# Sample every 2 seconds
while ps -p "$PID" > /dev/null 2>&1; do
    # Get CPU % for the given PID
    CPU=$(pidstat -p $PID 2 1 | grep Average | awk '{print $8}')

    # Output timestamp and CPU %
    norm=$(echo "$CPU / $CORES" | bc -l)
    echo "$TS,$norm" >> "$OUTFILE"

    # Increment simulated timestamp by +2
    TS=$((TS + 2))
done
