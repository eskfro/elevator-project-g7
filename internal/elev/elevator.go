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
const STUCK_TICKER_INTERVAL = 500 * time.Millisecond

const ORDER_TIMEOUT = 20 * time.Second
const STUCK_TIMEOUT = 10 * time.Second
const HEARTBEAT_TIMEOUT = 501 * time.Millisecond
const BETWEEN_FLOORS_TIMEOUT = 7000 * time.Millisecond

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
	ElevId     int
	Floor      int
	ButtonType elevio.ButtonType
}

type OrderTablePacket struct {
	Id         int
	Version    uint64
	OrderTable OrderTable
}

// Info about single elevator
type ElevatorPhysicalInfo struct {
	Id              int
	Floor           int
	PrimaryId       int
	Ip              string
	Role            ElevatorRole
	MotorDir        elevio.MotorDirection
	Movement        ElevatorMovement
	Obstructed      bool
	LocalOrderTable [N_FLOORS][N_BUTTONS]bool
}

// ELEVATOR
type Elevator struct {
	PhysicalInfo   ElevatorPhysicalInfo
	OrderTable     OrderTable
	AllOrderTables AllOrderTables
	AliveList      AliveList
	NumElevs       int
}

func CreateElevator(_Id int, _Port int, _Ip string) Elevator {
	elevator := Elevator{PhysicalInfo: createPhysicalElevator(_Id, _Ip)}
	elevator.AliveList[_Id] = elevator.PhysicalInfo
	elevator.NumElevs = 1
	return elevator
}

func createPhysicalElevator(_Id int, _Ip string) ElevatorPhysicalInfo {
	physicalElevator := ElevatorPhysicalInfo{
		Id:         _Id,
		Role:       ER_Backup,
		Ip:         _Ip,
		PrimaryId:  INVALID_PRIMARY_ID,
		Floor:      elevio.GetFloor(),
		MotorDir:   elevio.MD_Stop,
		Movement:   EM_Idle,
		Obstructed: elevio.GetObstruction(),
	}
	return physicalElevator
}
