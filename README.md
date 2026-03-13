# elevator-project-g7
Elevator project where the goal is to make fault tolerant code for a distributed system. Under no circumstances orders should not be lost.

### Info
- Course: TTK4154 Real Time Programming
- Group: 7 
- Semester: 2026V
- Collaborators: Marius and Eskil

### Dependencies 
- Golang 1.20

### Build
- make build

### Run simulator
- ./out id hardwarePort ordertablePort heartbeatPort
- ./SimElevatorServer --port hardwarePort <br>
or <br>
- make sim0, make sim1, make sim2
- make simall

### Kill simulator 
- make kill

