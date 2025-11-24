#!/bin/bash
DEVICE=$1
OUTFILE=$2

if [ -z "$DEVICE" ] || [ -z "$OUTFILE" ]; then
    echo "Usage: $0 <device-name> <output-file.csv>"
    exit 1
fi

if ! command -v iostat >/dev/null 2>&1; then
    echo "Error: iostat not found (install sysstat package)"
    exit 1
fi

echo "timestamp,%util" > "$OUTFILE"

# Run iostat and parse output
iostat -x 2 |
awk -v dev="$DEVICE" -v file="$OUTFILE" '
    BEGIN {
        timestamp = 0
        skip_first = 1
    }
    # skip CPU & header lines
    /^avg-cpu/ {next}
    /^Device/  {next}
    # match our device
    $1 == dev {
        # skip first iteration (initial state)
        if (skip_first) {
            skip_first = 0
            next
        }
        # print timestamp + util
        printf "%s,%s\n", timestamp, $(NF) >> file
        fflush(file)
        # increment timestamp by 2 seconds
        timestamp += 2
    }
'