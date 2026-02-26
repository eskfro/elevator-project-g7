#!/bin/bash

SCRIPT_DIR=$(dirname "$0")

cd "$SCRIPT_DIR/.."

# DEFINE PORTS (hardware, RcvPort1, RcvPort2)

HW_1=15657
HW_2=15658
HW_3=15659

HB_1=11311
HB_2=11311
HB_3=11311

OT_1=10311
OT_2=10311
OT_3=10311

# ELEVATOR ID'S

E1=0
E2=1
E3=2

echo "Starting All Simulators from $(pwd)..."
gnome-terminal --title="Sim Server $HW_1 [ $E1 ]" -- bash -c "./SimElevatorServer --port $HW_1; exec bash" &
sleep 1
gnome-terminal --title="Sim Server $HW_2 [ $E2 ]" -- bash -c "./SimElevatorServer --port $HW_2; exec bash" &
sleep 1
gnome-terminal --title="Sim Server $HW_3 [ $E3 ]" -- bash -c "./SimElevatorServer --port $HW_3; exec bash" &

sleep 2 

gnome-terminal --title="Elevator $E1" -- bash -c "./out $E1 $HW_1 $HB_1 $OT_1 ; exec bash" &
sleep 1
gnome-terminal --title="Elevator $E2" -- bash -c "./out $E2 $HW_2 $HB_2 $OT_2 ; exec bash" &
sleep 1
gnome-terminal --title="Elevator $E3" -- bash -c "./out $E3 $HW_3 $HB_3 $OT_3 ; exec bash" &

wait