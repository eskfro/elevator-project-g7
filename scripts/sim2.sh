#!/bin/bash

SCRIPT_DIR=$(dirname "$0")

cd "$SCRIPT_DIR/.."

# DEFINE PORTS (hardware, RcvPort1, RcvPort2)

HW_1=15659

HB_1=11311

OT_1=10311


# ELEVATOR ID'S

E1=2

echo "Starting One Simulator from $(pwd)..."
gnome-terminal --title="Sim Server $HW_1 [ $E1 ]" -- bash -c "./SimElevatorServer --port $HW_1; exec bash" &

sleep 2 

gnome-terminal --title="Elevator $E1" -- bash -c "./out $E1 $HW_1 $HB_1 $OT_1 ; exec bash" &

wait