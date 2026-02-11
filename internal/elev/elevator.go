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
	e := ElevatorPhysicalInfo{}
	e.Id = _Id
	e.Port = _Port
	e.NumFloors = N_FLOORS
	e.Floor = elevio.GetFloor()
	e.MotorDir = elevio.MD_Stop
	e.State = EM_Idle

	e.DoorTimer = timer.New(DOOR_OPEN_TIME)

	for f := 0; f < N_FLOORS; f++ {
		for b := 0; b < N_BUTTONS; b++ {
			e.LocalOrderTable[f][b] = false
		}
	}

	return e
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
