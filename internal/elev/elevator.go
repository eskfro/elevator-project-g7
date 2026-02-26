package elev

import (
	"elevator-project-g7/internal/elevio"
	"fmt"
	"time"
)

const N_FLOORS = 4
const N_BUTTONS = 3
const DOOR_OPEN_TIME = 3 * time.Second
const HEARTBEAT_TIMEOUT = 3 * time.Second
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
	ER_Backup  ElevatorRole = 1
	ER_Primary ElevatorRole = 2
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

type AliveListPacket struct {
	Id        int
	AliveList AliveList
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
	Floor           int
	PrimaryId       int
	Ip              string
	Role            ElevatorRole
	MotorDir        elevio.MotorDirection
	State           ElevatorMovement
	Obstructed      bool
	LocalOrderTable [N_FLOORS][N_BUTTONS]bool
}

// ELEVATOR
type Elevator struct {
	PhysicalInfo   ElevatorPhysicalInfo // Info about the single elevator
	OrderTable     OrderTable
	AllOrderTables AllOrderTables //Primary
	AliveList      AliveList
	NumElevs       int
}

func CreateElevator(_Id int, _Port int, _Ip string) Elevator {
	e := Elevator{

		PhysicalInfo: CreatePhysicalElevator(_Id, _Ip),
	}

	e.AliveList[_Id] = e.PhysicalInfo

	return e
}

func CreatePhysicalElevator(_Id int, _Ip string) ElevatorPhysicalInfo {
	pe := ElevatorPhysicalInfo{
		Id:         _Id,
		Role:       ER_Backup,
		Ip:         _Ip,
		Floor:      elevio.GetFloor(),
		MotorDir:   elevio.MD_Stop,
		State:      EM_Idle,
		Obstructed: false,
	}
	return pe
}

func PrintElevatorInit(id int, port int) {
	fmt.Printf("elevator starting | id = %d | port = %d\n", id, port)

}

func PrintElevatorInfo(elevator Elevator) {

	Active := "  #  "
	Inactive := "  -  "

	LOT := elevator.PhysicalInfo.LocalOrderTable

	fmt.Printf("--------------------------------\n")
	fmt.Printf("ELEVATOR %d ", elevator.PhysicalInfo.Id)
	fmt.Printf(" [ " + elevatorRoleToString(elevator.PhysicalInfo.Role) + " ] ")
	fmt.Printf(" < " + elevator.PhysicalInfo.Ip + " > \n")
	fmt.Printf("--------------------------------\n")

	// Print floor row
	fmt.Printf("Floor     |")
	for floor := 0; floor < N_FLOORS; floor++ {
		fmt.Printf("  %d  ", floor)
	}
	fmt.Printf("|\n")

	// Hall Up
	fmt.Printf("Hall Up	  |")
	for floor := 0; floor < N_FLOORS; floor++ {
		s := Inactive
		if LOT[floor][elevio.BT_HallUp] {
			s = Active
		}
		fmt.Printf(s)
	}
	fmt.Printf("|\n")

	// Hall Up
	fmt.Printf("Hall Down |")
	for floor := 0; floor < N_FLOORS; floor++ {
		s := Inactive
		if LOT[floor][elevio.BT_HallDown] {
			s = Active
		}
		fmt.Printf(s)
	}
	fmt.Printf("|\n")

	// Hall Up
	fmt.Printf("Cab       |")
	for floor := 0; floor < N_FLOORS; floor++ {
		s := Inactive
		if LOT[floor][elevio.BT_Cab] {
			s = Active
		}
		fmt.Printf(s)
	}
	fmt.Printf("|\n")

	fmt.Printf("--------------------------------\n")
	fmt.Printf("ALIVELIST\n")
	fmt.Printf("--------------------------------\n")

	for elevId := 0; elevId < N_MAX_ELEVS; elevId++ {
		fmt.Printf(" %d ", elevId)
	}
	fmt.Printf("\n--------------------------------\n")
	for elevId := 0; elevId < N_MAX_ELEVS; elevId++ {
		if elevator.AliveList[elevId].Role != ER_Dead {
			fmt.Printf(" 1 ")
		} else {
			fmt.Printf(" 0 ")
		}
	}
	fmt.Printf("\n--------------------------------\n\n\n")

}

func elevatorRoleToString(elevRole ElevatorRole) string {
	var roleString string

	switch elevRole {
	case ER_Backup:
		roleString = "Backup"
	case ER_Primary:
		roleString = "Primary"
	case ER_Dead:
		roleString = "Dead"
	}
	return roleString
}
