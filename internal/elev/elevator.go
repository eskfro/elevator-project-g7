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

// ================= ENUM TYPES ===============================

type OrderStatus uint8

const (
	OS_NO_ORDER  OrderStatus = 0
	OS_REQUESTED OrderStatus = 1
	OS_CONFIRMED OrderStatus = 2
	OS_CLEAR     OrderStatus = 3
)

type ElevatorMovement uint8

const (
	EM_Idle     ElevatorMovement = 0
	EM_Moving   ElevatorMovement = 1
	EM_DoorOpen ElevatorMovement = 2
)

type ElevatorRole uint8

const (
	ER_Backup  ElevatorRole = 0
	ER_Primary ElevatorRole = 1
	ER_Init    ElevatorRole = 2
)

// ================ Helper Structs ===========================

type Order struct {
	ElevatorNumber int
	Floor          int
	ButtonType     elevio.ButtonType
}

type RoleIdPair struct {
	Id   int
	Role ElevatorRole
}

type WorldView struct {
	// E: hva skal vi med map her?
	// E: jeg føler map er litt lite forutsigbart med tenke på at vi skal sende det over en 1024 byte buffer.
	// Men idk assa
	//OrderTable      map[Order]OrderStatus     //Bytta navn fra OrderTable men angrer ekstremt😔 Har ikkje tid å endre tilbake no


	
	OrderTable      [N_MAX_ELEVS][N_FLOORS][N_BUTTONS]OrderStatus //E: vi får diskutere denne :)
	LocalOrderTable [N_FLOORS][N_BUTTONS]bool                     //Local orders assigned by primary
}

// ============ ELEVATOR CORE =======================================

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
	Id            uint16
	Port          uint16
	PhysicalInfo  ElevatorPhysicalInfo
	WorldView     WorldView
	AllWorldViews [N_MAX_ELEVS]WorldView  //Primary
	AliveList     [N_MAX_ELEVS]RoleIdPair //Primary
	NumElevs      uint8
}

func CreateElevator(_Id uint16, _Port uint16) Elevator {
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
func InitPhysicalElevator(ip string, port uint16) {

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

func PrintElevatorInit(id uint16, port uint16) {
	fmt.Printf("elevator starting | id = %d | port = %d\n", id, port)

}
