package requests

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"fmt"
)

type DirnMovementPair struct {
	Direction elevio.MotorDirection
	Movement  elev.ElevatorMovement
}

func RequestAbove(LocalOrderTable [elev.N_FLOORS][elev.N_BUTTONS]bool, currentFloor int) bool {

	for f := currentFloor + 1; f < elev.N_FLOORS; f++ {
		for b := 0; b < elev.N_BUTTONS; b++ {
			if LocalOrderTable[f][b] {
				return true
			}
		}
	}
	return false
}

func RequestBelow(LocalOrderTable [elev.N_FLOORS][elev.N_BUTTONS]bool, currentFloor int) bool {

	for f := 0; f < currentFloor; f++ {
		for b := 0; b < elev.N_BUTTONS; b++ {
			if LocalOrderTable[f][b] {
				return true
			}
		}
	}
	return false
}

func RequestHere(LocalOrderTable [elev.N_FLOORS][elev.N_BUTTONS]bool, currentFloor int) bool {

	for b := 0; b < elev.N_BUTTONS; b++ {
		if LocalOrderTable[currentFloor][b] {
			return true
		}
	}
	return false
}

func ChooseDirection(LocalOrderTable [elev.N_FLOORS][elev.N_BUTTONS]bool, currentFloor int, dir elevio.MotorDirection) DirnMovementPair {
	switch dir {
	case elevio.MD_Up:
		if RequestAbove(LocalOrderTable, currentFloor) {
			return DirnMovementPair{
				Direction: elevio.MD_Up,
				Movement:  elev.EM_Moving,
			}
		} else if RequestHere(LocalOrderTable, currentFloor) {
			return DirnMovementPair{
				Direction: elevio.MD_Down,
				Movement:  elev.EM_DoorOpen,
			}
		} else if RequestBelow(LocalOrderTable, currentFloor) {
			return DirnMovementPair{
				Direction: elevio.MD_Down,
				Movement:  elev.EM_Moving,
			}
		} else {
			return DirnMovementPair{
				Direction: elevio.MD_Stop,
				Movement:  elev.EM_Idle,
			}
		}
	case elevio.MD_Down:
		if RequestBelow(LocalOrderTable, currentFloor) {
			return DirnMovementPair{
				Direction: elevio.MD_Down,
				Movement:  elev.EM_Moving,
			}
		} else if RequestHere(LocalOrderTable, currentFloor) {
			return DirnMovementPair{
				Direction: elevio.MD_Up,
				Movement:  elev.EM_DoorOpen,
			}
		} else if RequestAbove(LocalOrderTable, currentFloor) {
			return DirnMovementPair{
				Direction: elevio.MD_Up,
				Movement:  elev.EM_Moving,
			}
		} else {
			return DirnMovementPair{
				Direction: elevio.MD_Stop,
				Movement:  elev.EM_Idle,
			}
		}
	case elevio.MD_Stop:
		if RequestHere(LocalOrderTable, currentFloor) {
			return DirnMovementPair{
				Direction: elevio.MD_Stop,
				Movement:  elev.EM_DoorOpen,
			}
		} else if RequestAbove(LocalOrderTable, currentFloor) {
			return DirnMovementPair{
				Direction: elevio.MD_Up,
				Movement:  elev.EM_Moving,
			}
		} else if RequestBelow(LocalOrderTable, currentFloor) {
			return DirnMovementPair{
				Direction: elevio.MD_Down,
				Movement:  elev.EM_Moving,
			}
		} else {
			return DirnMovementPair{
				Direction: elevio.MD_Stop,
				Movement:  elev.EM_Idle,
			}
		}

	default:
		return DirnMovementPair{
			Direction: elevio.MD_Stop,
			Movement:  elev.EM_Idle,
		}
	}
}

func ShouldStop(LocalOrderTable [elev.N_FLOORS][elev.N_BUTTONS]bool, currentFloor int, dir elevio.MotorDirection) bool {
	switch dir {
	case elevio.MD_Down:
		return LocalOrderTable[currentFloor][elevio.BT_HallDown] ||
			LocalOrderTable[currentFloor][elevio.BT_Cab] ||
			!RequestBelow(LocalOrderTable, currentFloor)
	case elevio.MD_Up:
		return LocalOrderTable[currentFloor][elevio.BT_HallUp] ||
			LocalOrderTable[currentFloor][elevio.BT_Cab] ||
			!RequestAbove(LocalOrderTable, currentFloor)

	default:
		fmt.Println("ShouldStop case default")
		return true
	}
}

func ShouldClearImmediately(currentFloor int, dir elevio.MotorDirection, btnFloor int, btnType elevio.ButtonType) bool {
	sameFloor := currentFloor == btnFloor
	btnTypeMatchDir := (dir == elevio.MD_Up && btnType == elevio.BT_HallUp) ||
		(dir == elevio.MD_Down && btnType == elevio.BT_HallDown) ||
		(dir == elevio.MD_Stop) ||
		btnType == elevio.BT_Cab

	return sameFloor && btnTypeMatchDir
}

func ClearCurrentFloor(LocalOrderTable [elev.N_FLOORS][elev.N_BUTTONS]bool,
	currentFloor int,
	dir elevio.MotorDirection) [elev.N_FLOORS][elev.N_BUTTONS]bool {
	// Shorter name for clarity
	LOT := LocalOrderTable
	cf := currentFloor

	// Clear the cab order
	LOT[cf][elevio.BT_Cab] = false

	switch dir {

	case elevio.MD_Up:
		if !RequestAbove(LOT, cf) && !LOT[cf][elevio.BT_HallUp] {
			LOT[cf][elevio.BT_HallDown] = false
		}
		LOT[cf][elevio.BT_HallUp] = false

	case elevio.MD_Down:
		if !RequestBelow(LOT, cf) && !LOT[cf][elevio.BT_HallDown] {
			LOT[cf][elevio.BT_HallUp] = false
		}
		LOT[cf][elevio.BT_HallDown] = false

	default:
		LOT[cf][elevio.BT_HallUp] = false
		LOT[cf][elevio.BT_HallDown] = false
	}
	// New Local Order Table after change
	// Set it in the movement.go logic?
	return LOT
}

func PrintLOT(LocalOrderTable [elev.N_FLOORS][elev.N_BUTTONS]bool) {
	for i := 0; i < elev.N_FLOORS; i++ {
		for j := 0; j < elev.N_BUTTONS; j++ {
			if LocalOrderTable[i][j] {
				fmt.Printf("  #   ")
			} else {
				fmt.Printf("  -   ")
			}
		}
		fmt.Println()
	}
}
