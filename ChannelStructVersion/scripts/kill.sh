#!/bin/bash

echo "Stopping all elevator processes and simulators..."

pkill -f "./out "
pkill -f "SimElevatorServer"

