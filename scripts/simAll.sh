#!/bin/bash

SCRIPT_DIR=$(dirname "$0")
cd "$SCRIPT_DIR/.."

# Grid Settings (Adjust these if windows overlap or gaps are too big)
W=100  # Width in characters
H=30  # Height in characters
X0=0   # Left column X
X1=600 # Middle column X (pixels)
X2=1200 # Right column X (pixels)
Y_TOP=0     # Top row Y
Y_BOT=500   # Bottom row Y (pixels)

# Ports and IDs
HW_1=15657; HW_2=15658; HW_3=15659
HB_1=11311; HB_2=11311; HB_3=11311
OT_1=10311; OT_2=10311; OT_3=10311
E1=0; E2=1; E3=2

echo "Launching Grid Layout..."

# --- BOTTOM ROW: SIMULATORS ---
gnome-terminal --geometry="${W}x${H}+${X0}+${Y_BOT}" --title="Sim Server $HW_1" -- bash -c "./SimElevatorServer --port $HW_1; exec bash" &
gnome-terminal --geometry="${W}x${H}+${X1}+${Y_BOT}" --title="Sim Server $HW_2" -- bash -c "./SimElevatorServer --port $HW_2; exec bash" &
gnome-terminal --geometry="${W}x${H}+${X2}+${Y_BOT}" --title="Sim Server $HW_3" -- bash -c "./SimElevatorServer --port $HW_3; exec bash" &

sleep 2

# --- TOP ROW: ELEVATORS ---
gnome-terminal --geometry="${W}x${H}+${X0}+${Y_TOP}" --title="Elevator $E1" -- bash -c "./out $E1 $HW_1 $HB_1 $OT_1 ; exec bash" &
sleep 1
gnome-terminal --geometry="${W}x${H}+${X1}+${Y_TOP}" --title="Elevator $E2" -- bash -c "./out $E2 $HW_2 $HB_2 $OT_2 ; exec bash" &
sleep 1
gnome-terminal --geometry="${W}x${H}+${X2}+${Y_TOP}" --title="Elevator $E3" -- bash -c "./out $E3 $HW_3 $HB_3 $OT_3 ; exec bash" &

wait