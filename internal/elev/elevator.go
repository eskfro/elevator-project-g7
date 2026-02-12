package elev

import (
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/rolemanager"
	"elevator-project-g7/internal/timer"
	"fmt"
	"time"
)

const N_FLOORS = 4
const N_BUTTONS = 3
const DOOR_OPEN_TIME = time.Second * 3
const N_MAX_ELEVS = 9

// TODO: vi må fikse alle import cycles.
// Tror vi må prøve å lage alle elementene til Elevator her egentlig.

// ================= Orders ===============================

type Order struct {
	ElevatorNumber int
	Floor          int
	ButtonType     elevio.ButtonType
}

type OrderStatus int

const (
	OS_NO_ORDER  OrderStatus = 0
	OS_REQUESTED OrderStatus = 1
	OS_CONFIRMED OrderStatus = 2
	OS_CLEAR     OrderStatus = 3
)

// ================ Elevator WorldView(s) =======================

type WorldView struct {
	OrderTable      [N_MAX_ELEVS][N_FLOORS][N_BUTTONS]OrderStatus //Orders for all elevators
	LocalOrderTable [N_FLOORS][N_BUTTONS]bool                     //Local orders assigned by primary
}

// ==================== Elevator Stuff =============================

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
	WorldView     WorldView
	Role          rolemanager.ElevatorRole
	Id            int
	Port          int
	PhysicalInfo  ElevatorPhysicalInfo
	AllWorldViews [N_MAX_ELEVS]WorldView //Primary
}

func CreateElevator(_Id int, _Port int) Elevator {
	e := Elevator{
		Id:            _Id,
		Port:          _Port,
		Role:          rolemanager.ER_Init,
		PhysicalInfo:  CreatePhysicalElevator(),
		WorldView:     CreateWorldView(),
		AllWorldViews: CreateAllWorldViews(), //Primary
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
func CreateWorldView() WorldView {
	wv := WorldView{}

	//Init OrderTable
	for n_e := 0; n_e < N_MAX_ELEVS; n_e++ {
		for f := 0; f < N_MAX_ELEVS; f++ {
			for b := 0; b < N_MAX_ELEVS; b++ {
				wv.OrderTable[n_e][f][b] = OS_NO_ORDER
			}
		}
	}

	//Init LocalOrderTable
	for f := 0; f < N_MAX_ELEVS; f++ {
		for b := 0; b < N_MAX_ELEVS; b++ {
			wv.LocalOrderTable[f][b] = false
		}
	}

	return wv
}

func CreateAllWorldViews() [N_MAX_ELEVS]WorldView {
	var allWorldViews [N_MAX_ELEVS]WorldView
	for i := 0; i < N_MAX_ELEVS; i++ {
		allWorldViews[i] = CreateWorldView()
	}
	return allWorldViews
}
