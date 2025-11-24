#!/bin/bash

# === CONFIGURATION ===
CLIENT="go run ./client/main.go"
IMAGE_SIZE=64

# START VALUES
CLIENTS=5
STEP_CLIENT=5
MAX_CLIENTS=200
TESTDURATION=10s

echo "---- CPU Bottleneck Test (Increasing Concurrency) ----"
echo "Image Size: $IMAGE_SIZE px"

# === BEGIN TEST LOOP ===

for clients in $(seq $CLIENTS $STEP_CLIENT $MAX_CLIENTS)
do
    echo ">>> Testing with $clients clients..."

    # Run client load test
    $CLIENT \
        -duration=$TESTDURATION \
        -concurrent=$clients \
        -image-size=$IMAGE_SIZE \
        -test=cpu
done

echo "---- All tests completed ----"
