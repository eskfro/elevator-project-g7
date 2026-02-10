package elev

import (
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/timer"
	"fmt"
	"time"
)

const N_FLOORS = 4
const N_BUTTONS = 3
const DOOR_OPEN_TIME = time.Second * 3

type ElevatorMovement int

const (
	EM_Idle     ElevatorMovement = 0
	EM_Moving   ElevatorMovement = 1
	EM_DoorOpen ElevatorMovement = 2
)

type ElevatorPhysicalInfo struct {
	Id              int
	Port            int
	Floor           int
	LocalOrderTable [N_FLOORS][N_BUTTONS]bool
	MotorDir        elevio.MotorDirection
	State           ElevatorMovement
	NumFloors       int
	Obstructed      bool
	DoorTimer       *timer.Timer
}

func CreateElevator(_Id int, _Port int) ElevatorPhysicalInfo {
	Elev := ElevatorPhysicalInfo{}
	Elev.Id = _Id
	Elev.Port = _Port
	Elev.NumFloors = N_FLOORS
	Elev.Floor = elevio.GetFloor()
	Elev.MotorDir = elevio.MD_Stop
	Elev.State = EM_Idle

	Elev.DoorTimer = timer.New(0 * time.Second)

	for f := 0; f < N_FLOORS; f++ {
		for b := 0; b < N_BUTTONS; b++ {
			Elev.LocalOrderTable[f][b] = false
		}
	}

	return Elev
}

func InitPhysicalElevator(ip string, port int) {

	elevio.Init(fmt.Sprintf("localhost:%d", port), N_FLOORS)

	elevio.SetMotorDirection(elevio.MD_Down)
	for elevio.GetFloor() == -1 {
	}
	elevio.SetMotorDirection(elevio.MD_Stop)

	for elevio.GetObstruction() {
		elevio.SetDoorOpenLamp(true)
	}
	elevio.SetDoorOpenLamp(false)
	elevio.SetFloorIndicator(elevio.GetFloor())
}
