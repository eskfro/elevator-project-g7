package elev

import (
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/ordercontrol"
	"elevator-project-g7/internal/rolemanager"
	"elevator-project-g7/internal/timer"
	"fmt"
	"time"
)

const N_FLOORS = 4
const N_BUTTONS = 3
const DOOR_OPEN_TIME = time.Second * 3
const N_MAX_ELEVS = 9

type ElevatorMovement int

const (
	EM_Idle     ElevatorMovement = 0
	EM_Moving   ElevatorMovement = 1
	EM_DoorOpen ElevatorMovement = 2
)

type ElevatorPhysicalInfo struct {
	Floor      int
	MotorDir   elevio.MotorDirection
	State      ElevatorMovement
	NumFloors  int
	Obstructed bool
	DoorTimer  *timer.Timer
}

type Elevator struct {
	WorldView     ordercontrol.WorldView
	Role          rolemanager.Role
	Id            int
	Port          int
	PhysicalInfo  ElevatorPhysicalInfo
	AllWorldViews [N_MAX_ELEVS]ordercontrol.WorldView //Primary
}

func CreateElevator(_Id int, _Port int) Elevator {
	e := Elevator{
		Id:            _Id,
		Port:          _Port,
		Role:          rolemanager.ROLE_Backup,
		PhysicalInfo:  CreatePhysicalElevator(),
		WorldView:     ordercontrol.CreateWorldView(),
		AllWorldViews: ordercontrol.CreateAllWorldViews(), //Primary
	}

	return e
}

func CreatePhysicalElevator() ElevatorPhysicalInfo {
	pe := ElevatorPhysicalInfo{
		NumFloors: N_FLOORS,
		Floor:     elevio.GetFloor(),
		MotorDir:  elevio.MD_Stop,
		State:     EM_Idle,
		DoorTimer: timer.New(DOOR_OPEN_TIME),
	}
	return pe
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
