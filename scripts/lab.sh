#!/bin/bash

if [ -z "$1" ]; then
    echo "Usage: $0 <elevator_id>"
    exit 1
fi

# Window Dimensions & Positions
W=70   # Width in characters
H=25   # Height in characters
X0=0   # Left column X
Y_BOT=500  # Bottom row Y (pixels)

E_ID=$1
SCRIPT_DIR=$(dirname "$0")
cd "$SCRIPT_DIR/.."

# Port Calculation
HW_PORT=15657 # hardware port, needs to mach the simulation script port
HB_PORT=11311 # heartbeat port -> constant for all elevs
OT_PORT=10411 # Port for primary TCP if this becomes primary. Elevator ID is when elevator is inited. It is actually different for OrderTable things
PP_ROLE=1 # 1 = Master / 0 = Slave (called from master program)



# --- DUPLICATE CHECK ---
# Check if an elevator with this ID is already running
if pgrep -f "./out $E_ID " > /dev/null; then
    echo "Error: Elevator $E_ID is already running. Close it before restarting."
    exit 1
fi

# --- ELEVATOR CLIENT ---
echo "Starting Elevator $E_ID (HW: $HW_PORT, HB: $HB_PORT, OT: $OT_PORT)..."
gnome-terminal --geometry="${W}x${H}+${X0}+${Y_BOT}" --title="Elevator $E_ID" -- bash -c "./out $E_ID $HW_PORT $HB_PORT $OT_PORT $PP_ROLE; exec bash" &
