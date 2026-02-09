package requests

import (
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/movement"
)

func RequestAbove(LocalOrderTable [][]bool, currentFloor int) bool {

	numFloors := len(LocalOrderTable)
	numButtons := len(LocalOrderTable[0])

	for f := currentFloor + 1; f < numFloors; f++ {
		for b := 0; b < numButtons; b++ {
			if LocalOrderTable[f][b] {
				return true
			}
		}
	}
	return false
}

func RequestBelow(LocalOrderTable [][]bool, currentFloor int) bool {

	numButtons := len(LocalOrderTable[0])

	for f := 0; f < currentFloor; f++ {
		for b := 0; b < numButtons; b++ {
			if LocalOrderTable[f][b] {
				return true
			}
		}
	}
	return false
}

func RequestHere(LocalOrderTable [][]bool, currentFloor int) bool {

	numButtons := len(LocalOrderTable[0])
	for b := 0; b < numButtons; b++ {
		if LocalOrderTable[currentFloor][b] {
			return true
		}
	}
	return false
}

func ChooseDirection(LocalOrderTable [][]bool, currentFloor int, dir elevio.MotorDirection) movement.DirnMovementPair {
	switch dir {
	case elevio.MD_Up:
		if RequestAbove(LocalOrderTable, currentFloor) {
			return movement.DirnMovementPair{
				Direction: elevio.MD_Up,
				Movement:  movement.EM_Moving,
			}
		} else if RequestHere(LocalOrderTable, currentFloor) {
			return movement.DirnMovementPair{
				Direction: elevio.MD_Down,
				Movement:  movement.EM_DoorOpen,
			}
		} else if RequestBelow(LocalOrderTable, currentFloor) {
			return movement.DirnMovementPair{
				Direction: elevio.MD_Down,
				Movement:  movement.EM_Moving,
			}
		} else {
			return movement.DirnMovementPair{
				Direction: elevio.MD_Stop,
				Movement:  movement.EM_Idle,
			}
		}
	case elevio.MD_Down:
		if RequestBelow(LocalOrderTable, currentFloor) {
			return movement.DirnMovementPair{
				Direction: elevio.MD_Down,
				Movement:  movement.EM_Moving,
			}
		} else if RequestHere(LocalOrderTable, currentFloor) {
			return movement.DirnMovementPair{
				Direction: elevio.MD_Up,
				Movement:  movement.EM_DoorOpen,
			}
		} else if RequestAbove(LocalOrderTable, currentFloor) {
			return movement.DirnMovementPair{
				Direction: elevio.MD_Up,
				Movement:  movement.EM_Moving,
			}
		} else {
			return movement.DirnMovementPair{
				Direction: elevio.MD_Stop,
				Movement:  movement.EM_Idle,
			}
		}
	case elevio.MD_Stop:
		if RequestHere(LocalOrderTable, currentFloor) {
			return movement.DirnMovementPair{
				Direction: elevio.MD_Stop,
				Movement:  movement.EM_DoorOpen,
			}
		} else if RequestAbove(LocalOrderTable, currentFloor) {
			return movement.DirnMovementPair{
				Direction: elevio.MD_Up,
				Movement:  movement.EM_Moving,
			}
		} else if RequestBelow(LocalOrderTable, currentFloor) {
			return movement.DirnMovementPair{
				Direction: elevio.MD_Down,
				Movement:  movement.EM_Moving,
			}
		} else {
			return movement.DirnMovementPair{
				Direction: elevio.MD_Stop,
				Movement:  movement.EM_Idle,
			}
		}

	default:
		return movement.DirnMovementPair{
			Direction: elevio.MD_Stop,
			Movement:  movement.EM_Idle,
		}
	}
}

func ShouldStop(LocalOrderTable [][]bool, currentFloor int, dir elevio.MotorDirection) bool {
	switch dir {
	case elevio.MD_Down:
		return LocalOrderTable[currentFloor][elevio.BT_HallDown] ||
			LocalOrderTable[currentFloor][elevio.BT_Cab] ||
			!RequestBelow(LocalOrderTable, currentFloor)
	case elevio.MD_Up:
		return LocalOrderTable[currentFloor][elevio.BT_HallUp] ||
			LocalOrderTable[currentFloor][elevio.BT_Cab] ||
			!RequestAbove(LocalOrderTable, currentFloor)
	// TOOD:
	// case elevio.MD_Stop ???

	default:
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

func ClearCurrentFloor(LocalOrderTable [][]bool, currentFloor int, dir elevio.MotorDirection) [][]bool {
	LOT, cf := LocalOrderTable, currentFloor
	// Clear the cab order
	LOT[cf][elevio.BT_Cab] = false

	switch dir {
	case elevio.MD_Up:
		if !RequestAbove(LOT, cf) && !LOT[cf][elevio.BT_HallUp] {
			LOT[cf][elevio.BT_HallDown] = false
		}
		LOT[cf][elevio.BT_HallUp] = false
		break
	case elevio.MD_Down:
		if !RequestBelow(LOT, cf) && !LOT[cf][elevio.BT_HallDown] {
			LOT[cf][elevio.BT_HallUp] = false
		}
		LOT[cf][elevio.BT_HallDown] = false
		break
	// TODO:
	// case elevio.MD_Stop ???
	default:
		LOT[cf][elevio.BT_HallUp] = false
		LOT[cf][elevio.BT_HallDown] = false
		break
	}
	// New Local Order Table after change
	// Use this function to calculate
	// and set the new LOT in movement.go
	return LOT
}
