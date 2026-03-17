package requests

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"fmt"
)

type DirnMovementPair struct {
	MotorDir elevio.MotorDirection
	Movement elev.ElevatorMovement
}

func RequestAbove(LocalOrderTable elev.LocalOrderTable, currentFloor int) bool {
	for floor := currentFloor + 1; floor < elev.N_FLOORS; floor++ {
		for btn := 0; btn < elev.N_BUTTONS; btn++ {
			if LocalOrderTable[floor][btn] {
				return true
			}
		}
	}
	return false
}

func RequestBelow(LocalOrderTable elev.LocalOrderTable, currentFloor int) bool {
	for floor := 0; floor < currentFloor; floor++ {
		for btn := 0; btn < elev.N_BUTTONS; btn++ {
			if LocalOrderTable[floor][btn] {
				return true
			}
		}
	}
	return false
}

func RequestHere(LocalOrderTable elev.LocalOrderTable, currentFloor int) bool {
	for btn := 0; btn < elev.N_BUTTONS; btn++ {
		if LocalOrderTable[currentFloor][btn] {
			return true
		}
	}
	return false
}

func ChooseDirection(
	localOrderTable elev.LocalOrderTable,
	currentFloor int,
	dir elevio.MotorDirection,
) DirnMovementPair {
	switch dir {
	case elevio.MD_Up:
		if RequestAbove(localOrderTable, currentFloor) {
			return DirnMovementPair{
				MotorDir: elevio.MD_Up,
				Movement: elev.EM_Moving,
			}
		} else if RequestHere(localOrderTable, currentFloor) {
			return DirnMovementPair{
				MotorDir: elevio.MD_Down,
				Movement: elev.EM_DoorOpen,
			}
		} else if RequestBelow(localOrderTable, currentFloor) {
			return DirnMovementPair{
				MotorDir: elevio.MD_Down,
				Movement: elev.EM_Moving,
			}
		} else {
			return DirnMovementPair{
				MotorDir: elevio.MD_Stop,
				Movement: elev.EM_Idle,
			}
		}
	case elevio.MD_Down:
		if RequestBelow(localOrderTable, currentFloor) {
			return DirnMovementPair{
				MotorDir: elevio.MD_Down,
				Movement: elev.EM_Moving,
			}
		} else if RequestHere(localOrderTable, currentFloor) {
			return DirnMovementPair{
				MotorDir: elevio.MD_Up,
				Movement: elev.EM_DoorOpen,
			}
		} else if RequestAbove(localOrderTable, currentFloor) {
			return DirnMovementPair{
				MotorDir: elevio.MD_Up,
				Movement: elev.EM_Moving,
			}
		} else {
			return DirnMovementPair{
				MotorDir: elevio.MD_Stop,
				Movement: elev.EM_Idle,
			}
		}
	case elevio.MD_Stop:
		if RequestHere(localOrderTable, currentFloor) {
			return DirnMovementPair{
				MotorDir: elevio.MD_Stop,
				Movement: elev.EM_DoorOpen,
			}
		} else if RequestAbove(localOrderTable, currentFloor) {
			return DirnMovementPair{
				MotorDir: elevio.MD_Up,
				Movement: elev.EM_Moving,
			}
		} else if RequestBelow(localOrderTable, currentFloor) {
			return DirnMovementPair{
				MotorDir: elevio.MD_Down,
				Movement: elev.EM_Moving,
			}
		} else {
			return DirnMovementPair{
				MotorDir: elevio.MD_Stop,
				Movement: elev.EM_Idle,
			}
		}

	default:
		return DirnMovementPair{
			MotorDir: elevio.MD_Stop,
			Movement: elev.EM_Idle,
		}
	}
}

func ShouldStop(
	localOrderTable elev.LocalOrderTable,
	currentFloor int,
	dir elevio.MotorDirection,
) bool {
	switch dir {
	case elevio.MD_Down:
		return localOrderTable[currentFloor][elevio.BT_HallDown] ||
			localOrderTable[currentFloor][elevio.BT_Cab] ||
			!RequestBelow(localOrderTable, currentFloor)
	case elevio.MD_Up:
		return localOrderTable[currentFloor][elevio.BT_HallUp] ||
			localOrderTable[currentFloor][elevio.BT_Cab] ||
			!RequestAbove(localOrderTable, currentFloor)

	default:
		fmt.Println("ShouldStop case default")
		return true
	}
}

func ClearCurrentFloor(
	localOrderTable elev.LocalOrderTable,
	currentFloor int,
	dir elevio.MotorDirection,
) (elev.LocalOrderTable, [elev.N_BUTTONS]bool) {

	var buttonsToClear [elev.N_BUTTONS]bool

	// Clear the cab order
	localOrderTable[currentFloor][elevio.BT_Cab] = false
	buttonsToClear[elevio.BT_Cab] = true

	switch dir {

	case elevio.MD_Up:
		if !RequestAbove(localOrderTable, currentFloor) && !localOrderTable[currentFloor][elevio.BT_HallUp] {
			localOrderTable[currentFloor][elevio.BT_HallDown] = false
			buttonsToClear[elevio.BT_HallDown] = true
		}
		localOrderTable[currentFloor][elevio.BT_HallUp] = false
		buttonsToClear[elevio.BT_HallUp] = true

	case elevio.MD_Down:
		if !RequestBelow(localOrderTable, currentFloor) && !localOrderTable[currentFloor][elevio.BT_HallDown] {
			localOrderTable[currentFloor][elevio.BT_HallUp] = false
			buttonsToClear[elevio.BT_HallUp] = true

		}
		localOrderTable[currentFloor][elevio.BT_HallDown] = false
		buttonsToClear[elevio.BT_HallDown] = true

	default:
		localOrderTable[currentFloor][elevio.BT_HallUp] = false
		localOrderTable[currentFloor][elevio.BT_HallDown] = false
		buttonsToClear[elevio.BT_HallUp] = true
		buttonsToClear[elevio.BT_HallDown] = true

	}
	return localOrderTable, buttonsToClear
}
