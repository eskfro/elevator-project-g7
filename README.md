# elevator-project-g7
Elevator project where the goal is to make fault tolerant code for a distributed system. Under no circumstances orders should be lost.

### Info
- Course: TTK4154 Real Time Programming
- Group: 7 
- Semester: 2026V
- Collaborators: Marius and Eskil

### Dependencies 
- Golang 1.20

### Building the binaries
- Elevator Program: make build
- Elevator Server: dmd -w -g src/sim_server.d src/timer_event.d -ofSimElevatorServer

### Run simulator at home
- Start program: ./out id hardwarePort ordertablePort heartbeatPort
- Start server: ./SimElevatorServer --port hardwarePort <br>
or <br>
- Start program: make sim0 / make sim1 / make sim2
- Start server: make simall

### Run simulator at the lab
- Start program: ./out id hardWarePort orderTablePort, heartbeatPort
- Start server: elevatorserver <br>
or <br>
- Start program: make lab0 / make lab1 / make lab2
- Start server: elevatorserver 


