#!/bin/bash

SCRIPT_DIR=$(dirname "$0")

echo "Launching 3 Elevator Nodes (IDs 0 to 2)..."

for i in {0..2}
do

    bash "$SCRIPT_DIR/sim.sh" $i
    
    sleep 0.5
done

echo "All nodes requested."