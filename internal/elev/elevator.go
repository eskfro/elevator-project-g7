package elev

import (
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/timer"
	"fmt"
	"time"
)

const N_FLOORS = 4
const N_BUTTONS = 3
const DOOR_OPEN_TIME = 3 * time.Second
const N_MAX_ELEVS = 9

// TODO: vi må fikse alle import cycles.
// Tror vi må prøve å lage alle elementene til Elevator her egentlig.
// Eler så lager vi en types modul -> E: jeg ønsker ikke det

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

// ================== Role Stuff ================================

type ElevatorRole int

const (
	ER_Backup  ElevatorRole = 0
	ER_Primary ElevatorRole = 1
	ER_Init    ElevatorRole = 2
)

// ==================== Elevator Stuff =============================

type ElevatorMovement int

const (
	EM_Idle     ElevatorMovement = 0
	EM_Moving   ElevatorMovement = 1
	EM_DoorOpen ElevatorMovement = 2
)

type WorldView struct {
	OrderTable      [N_MAX_ELEVS][N_FLOORS][N_BUTTONS]OrderStatus //Orders for all elevators
	LocalOrderTable [N_FLOORS][N_BUTTONS]bool                     //Local orders assigned by primary
}

type ElevatorPhysicalInfo struct {
	Floor      int
	MotorDir   elevio.MotorDirection
	State      ElevatorMovement
	NumFloors  int
	NumButtons int
	Obstructed bool
	DoorTimer  *timer.Timer
}

type Elevator struct {
	Role          ElevatorRole // E: Føler Role passer inn her egentlig
	Id            int
	Port          int
	PhysicalInfo  ElevatorPhysicalInfo
	WorldView     WorldView
	AllWorldViews [N_MAX_ELEVS]WorldView //Primary
	NumElevs      int
}

func CreateElevator(_Id int, _Port int) Elevator {
	elev := Elevator{
		Id:            _Id,
		Port:          _Port,
		Role:          ER_Init,
		PhysicalInfo:  CreatePhysicalElevator(),
		WorldView:     CreateWorldView(),
		AllWorldViews: CreateAllWorldViews(), //Primary
	}

	return elev
}

func CreatePhysicalElevator() ElevatorPhysicalInfo {
	pe := ElevatorPhysicalInfo{
		NumFloors:  N_FLOORS,
		NumButtons: N_BUTTONS,
		Floor:      elevio.GetFloor(),
		MotorDir:   elevio.MD_Stop,
		State:      EM_Idle,
		Obstructed: false,
		DoorTimer:  timer.New(DOOR_OPEN_TIME),
	}
	return pe
}

// Moves elevator to a floor
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
	for elevID := 0; elevID < N_MAX_ELEVS; elevID++ {
		for floor := 0; floor < N_FLOORS; floor++ {
			for btn := 0; btn < N_BUTTONS; btn++ {
				wv.OrderTable[elevID][floor][btn] = OS_NO_ORDER
			}
		}
	}

	//Init LocalOrderTable
	for floor := 0; floor < N_FLOORS; floor++ {
		for btn := 0; btn < N_BUTTONS; btn++ {
			wv.LocalOrderTable[floor][btn] = false
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

func PrintElevatorInit(id int, port int) {
	fmt.Printf("elevator starting | id = %d | port = %d\n", id, port)

}
