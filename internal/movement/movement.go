package movement

import (
	"elevator-project-g7/internal/elevio"
	"fmt"
)

/*
- ElevatorPhysicalInfo
*/

const N_FLOORS = 4

type ElevatorMovement int

const (
	EM_Idle     ElevatorMovement = 0
	EM_Moving   ElevatorMovement = 1
	EM_DoorOpen ElevatorMovement = 2
)

type ElevatorPhysicalInfo struct {
	Id        int
	Port      int
	Floor     int
	MotorDir  elevio.MotorDirection
	State     ElevatorMovement
	NumFloors int
}

func CreateElevator(_Id int, _Port int) ElevatorPhysicalInfo {
	Elev := ElevatorPhysicalInfo{}
	Elev.Id = _Id
	Elev.Port = _Port
	Elev.NumFloors = N_FLOORS
	Elev.Floor = elevio.GetFloor()
	Elev.MotorDir = elevio.MD_Stop
	Elev.State = EM_Idle

	return Elev
}

func InitPhysicalElevatorToFloor(ip string, port int, _initFloor int) {

	initFloor := _initFloor

	elevio.Init(fmt.Sprintf("localhost:%d", port), N_FLOORS)

	elevio.SetMotorDirection(elevio.MD_Down)
	for elevio.GetFloor() == -1 {
	}
	elevio.SetMotorDirection(elevio.MD_Stop)

	// Move elevator to initFloor
	if elevio.GetFloor() != initFloor {

		if elevio.GetFloor() < initFloor {
			elevio.SetMotorDirection(elevio.MD_Up)
		} else {
			elevio.SetMotorDirection(elevio.MD_Down)
		}
		for elevio.GetFloor() != initFloor {
		}
		elevio.SetMotorDirection(elevio.MD_Stop)
	}

	for elevio.GetObstruction() {
		elevio.SetDoorOpenLamp(true)
	}
	elevio.SetDoorOpenLamp(false)
	elevio.SetFloorIndicator(elevio.GetFloor())
}
