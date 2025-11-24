#!/bin/bash

# === CONFIGURATION ===
CLIENT="go run ./client/main.go"
IMAGE_SIZE=32000

# START VALUES
CLIENTS=1
STEP_CLIENT=2
MAX_CLIENTS=100
TESTDURATION=10s

echo "---- Upload Bottleneck Test (Increasing Concurrency) ----"
echo "Image Size: $IMAGE_SIZE px"
echo

# === BEGIN TEST LOOP ===
for clients in $(seq $CLIENTS $STEP_CLIENT $MAX_CLIENTS)
do
    echo ">>> Testing with $clients clients..."

    # Run client load test
    $CLIENT \
        -duration=$TESTDURATION \
        -concurrent=$clients \
        -image-size=$IMAGE_SIZE \
        -test=upload
done
echo "---- All tests completed ----"
