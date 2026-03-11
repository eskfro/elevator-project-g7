package elev

import (
	"elevator-project-g7/internal/elevio"
	"fmt"
	"strings"
	"time"
)

const N_FLOORS = 4
const N_BUTTONS = 3
const N_MAX_ELEVS = 4

const DOOR_OPEN_TIME = 3000 * time.Millisecond
const HEARTBEAT_TIMEOUT = 500 * time.Millisecond
const PRIMARY_ELECTION_DELAY = 401 * time.Millisecond
const BCAST_INTERVAL_HB = 50 * time.Millisecond
const BCAST_INTERVAL_OT = 50 * time.Millisecond

const INVALID_ELEVATOR_ID = N_MAX_ELEVS + 1
const INVALID_PRIMARY_ID = N_MAX_ELEVS + 1

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
	ElevId     int
	Floor      int
	ButtonType elevio.ButtonType
}

type RoleIdPair struct {
	Id   int
	Role ElevatorRole
}

type OrderTablePacket struct {
	Id         int
	Version    uint64
	OrderTable OrderTable
}

type NetworkPacket struct {
	OrderTableP  OrderTablePacket
	PhysicalInfo ElevatorPhysicalInfo
}

// ============ ELEVATOR CORE =======================================

// Custom types
type LocalOrderTable [N_FLOORS][N_BUTTONS]bool
type OrderTable [N_MAX_ELEVS][N_FLOORS][N_BUTTONS]OrderStatus
type AllOrderTables [N_MAX_ELEVS]OrderTable
type AliveList [N_MAX_ELEVS]ElevatorPhysicalInfo
type ClearOrders [N_BUTTONS]Order

// Info about single elevator
type ElevatorPhysicalInfo struct {
	Id              int
	Floor           int
	PrimaryId       int
	PrimaryIp       string
	Ip              string
	Role            ElevatorRole
	MotorDir        elevio.MotorDirection
	Movement        ElevatorMovement
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
	e := Elevator{PhysicalInfo: CreatePhysicalElevator(_Id, _Ip)}
	e.AliveList[_Id] = e.PhysicalInfo
	e.NumElevs = 1
	return e
}

func CreatePhysicalElevator(_Id int, _Ip string) ElevatorPhysicalInfo {
	pe := ElevatorPhysicalInfo{
		Id:         _Id,
		Role:       ER_Backup,
		Ip:         _Ip,
		PrimaryId:  INVALID_ELEVATOR_ID,
		Floor:      elevio.GetFloor(),
		MotorDir:   elevio.MD_Stop,
		Movement:   EM_Idle,
		Obstructed: false,
	}
	return pe
}

func PrintElevatorInit(id int, port_HW int) {
	fmt.Printf("\nelevator starting | id = %d | port.Hardware = %d\n\n", id, port_HW)

}

func PrintElevatorInfo(elevator Elevator, uptime float64) {

	Active := "  #  "
	Inactive := "  -  "
	uptimeString := fmt.Sprintf("%.1f", uptime)

	LOT := elevator.PhysicalInfo.LocalOrderTable

	fmt.Printf("--------------------------------\n")
	fmt.Printf("ELEVATOR %d ", elevator.PhysicalInfo.Id)
	fmt.Printf(" [ " + ElevatorRoleToString(elevator.PhysicalInfo.Role) + " ] ")
	fmt.Printf(" < " + elevator.PhysicalInfo.Ip + " > |")
	fmt.Printf(" t = " + uptimeString + "s |")
	fmt.Printf(" primaryId = %d\n", elevator.PhysicalInfo.PrimaryId)
	fmt.Printf("--------------------------------\n")
	fmt.Printf("STATE = " + ElevatorMovementToString(elevator.PhysicalInfo.Movement) + "\n")
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
	fmt.Printf("ALIVELIST | NumElevs = %d\n", elevator.NumElevs)
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

func ElevatorRoleToString(elevRole ElevatorRole) string {
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

func ElevatorMovementToString(movement ElevatorMovement) string {
	var movementString string

	switch movement {
	case EM_DoorOpen:
		movementString = "DoorOpen"
	case EM_Moving:
		movementString = "Moving"
	case EM_Idle:
		movementString = "Idle"
	}
	return movementString

}

func (status OrderStatus) String() string {
	switch status {
	case OS_CLEAR:
		return "Clear"
	case OS_NO_ORDER:
		return "No order"
	case OS_REQUESTED:
		return "Requested"
	case OS_CONFIRMED:
		return "Confirmed"
	default:
		return "Unknown"
	}
}

func PrintOrderTableSlice(table OrderTable, elevID int) {
	OT_Slice := table[elevID]
	buttonNames := []string{"Hall Up", "Hall Down", "Cab"}

	fmt.Printf("--- Primary's Order Table page %d\n", elevID)

	// Print header: "Button / Floor" etterfulgt av alle etasjenumrene
	fmt.Printf("%-12s", "Button\\Floor")
	for f := 0; f < len(OT_Slice); f++ {
		fmt.Printf(" | Floor %-2d", f)
	}
	fmt.Println(" |")
	fmt.Println(strings.Repeat("-", 12+len(OT_Slice)*11))

	// Print én rad for hver knappetype
	for b := 0; b < len(buttonNames); b++ {
		fmt.Printf("%-12s", buttonNames[b])
		for f := 0; f < len(OT_Slice); f++ {
			status := OT_Slice[f][b]
			fmt.Printf(" | %-8s", status)
		}
		fmt.Println(" |")
	}
	fmt.Println(strings.Repeat("-", 12+len(OT_Slice)*11))
}
