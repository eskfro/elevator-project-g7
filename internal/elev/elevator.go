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
const N_MAX_ELEVS = 4

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
	ER_Dead    ElevatorRole = 0
	ER_Init    ElevatorRole = 1
	ER_Backup  ElevatorRole = 2
	ER_Primary ElevatorRole = 3
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

type OrderTablePacket struct {
	Id         int
	OrderTable OrderTable
}

// ============ ELEVATOR CORE =======================================

// Custom types
type LocalOrderTable [N_FLOORS][N_BUTTONS]bool
type OrderTable [N_MAX_ELEVS][N_FLOORS][N_BUTTONS]OrderStatus
type AllOrderTables [N_MAX_ELEVS]OrderTable
type AliveList [N_MAX_ELEVS]ElevatorPhysicalInfo

// Info about single elevator
type ElevatorPhysicalInfo struct {
	Id              int
	Port            int
	Floor           int
	Role            ElevatorRole
	MotorDir        elevio.MotorDirection
	State           ElevatorMovement
	Obstructed      bool
	DoorTimer       *timer.Timer //TODO: maybe remove
	LocalOrderTable [N_FLOORS][N_BUTTONS]bool
}

// ELEVATOR
type Elevator struct {
	PhysicalInfo   ElevatorPhysicalInfo // Info about the single elevator
	OrderTable     OrderTable
	AllOrderTables AllOrderTables //Primary
	AliveList      AliveList
	NumElevs       uint8
}

func CreateElevator(_Id uint16, _Port uint16) Elevator {
	e := Elevator{

		PhysicalInfo: CreatePhysicalElevator(_Id, _Port),
	}

	e.AliveList[_Id] = e.PhysicalInfo

	return e
}

func CreatePhysicalElevator(_Id uint16, _Port uint16) ElevatorPhysicalInfo {
	pe := ElevatorPhysicalInfo{
		Id:         _Id,
		Port:       _Port,
		Role:       ER_Init,
		Floor:      elevio.GetFloor(),
		MotorDir:   elevio.MD_Stop,
		State:      EM_Idle,
		Obstructed: false,
	}
	return pe
}

func PrintElevatorInit(id uint16, port uint16) {
	fmt.Printf("elevator starting | id = %d | port = %d\n", id, port)

}
