package elev

import (
	"elevator-project-g7/internal/elevio"
	"time"
)

// Physical config
const N_FLOORS = 4
const N_BUTTONS = 3
const N_MAX_ELEVS = 4

const DOOR_OPEN_TIME = 3000 * time.Millisecond
const PRIMARY_ELECTION_DELAY = 401 * time.Millisecond

const ORDER_TIMEOUT = 20 * time.Second
const STUCK_DOOR_TIMEOUT = 15 * time.Second
const IDLE_RESTART_TIMEOUT = 1000 * time.Second
const HEARTBEAT_TIMEOUT = 501 * time.Millisecond
const BETWEEN_FLOORS_TIMEOUT = 7000 * time.Millisecond

// Backup dont set orderTable from primary until primary gets orderTable from backup
const BACKUP_RCV_PRIMARYOT_TIMEOUT = 1000 * time.Millisecond

const INVALID_PRIMARY_ID = N_MAX_ELEVS + 1
const INVALID_ELEVATOR_ID = N_MAX_ELEVS + 1

// ==================================== CUSTOM TYPES

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

type LocalOrderTable [N_FLOORS][N_BUTTONS]bool
type OrderTable [N_MAX_ELEVS][N_FLOORS][N_BUTTONS]OrderStatus
type AllOrderTables [N_MAX_ELEVS]OrderTable
type AliveList [N_MAX_ELEVS]ElevatorPhysicalInfo
type ClearOrders [N_BUTTONS]Order

// ===================================== STRUCTS

type Order struct {
	ElevID     int
	Floor      int
	ButtonType elevio.ButtonType
}

type OrderTablePacket struct {
	ID         int
	Version    uint64
	OrderTable OrderTable
}

// Info about single elevator
type ElevatorPhysicalInfo struct {
	ID              int
	Floor           int
	PrimaryID       int
	IP              string
	Role            ElevatorRole
	MotorDir        elevio.MotorDirection
	Movement        ElevatorMovement
	Obstructed      bool
	LocalOrderTable LocalOrderTable
}

// ELEVATOR
type Elevator struct {
	PhysicalInfo   ElevatorPhysicalInfo
	OrderTable     OrderTable
	AllOrderTables AllOrderTables
	AliveList      AliveList
	NumElevs       int
}

func CreateElevator(_ID int, _Port int, _IP string) Elevator {
	elevator := Elevator{PhysicalInfo: createPhysicalElevator(_ID, _IP)}
	elevator.AliveList[_ID] = elevator.PhysicalInfo
	elevator.NumElevs = 1

	if elevio.GetObstruction() {
		elevator.PhysicalInfo.Obstructed = true
		elevio.SetDoorOpenLamp(true)
	}

	return elevator
}

func createPhysicalElevator(_ID int, _IP string) ElevatorPhysicalInfo {
	physicalElevator := ElevatorPhysicalInfo{
		ID:         _ID,
		Role:       ER_Backup,
		IP:         _IP,
		PrimaryID:  INVALID_PRIMARY_ID,
		Floor:      elevio.GetFloor(),
		MotorDir:   elevio.MD_Stop,
		Movement:   EM_Idle,
		Obstructed: false,
	}
	return physicalElevator
}
