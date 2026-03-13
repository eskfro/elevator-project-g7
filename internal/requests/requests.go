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

func ChooseDirection(LocalOrderTable elev.LocalOrderTable, currentFloor int, dir elevio.MotorDirection) DirnMovementPair {
	switch dir {
	case elevio.MD_Up:
		if RequestAbove(LocalOrderTable, currentFloor) {
			return DirnMovementPair{
				MotorDir: elevio.MD_Up,
				Movement: elev.EM_Moving,
			}
		} else if RequestHere(LocalOrderTable, currentFloor) {
			return DirnMovementPair{
				MotorDir: elevio.MD_Down,
				Movement: elev.EM_DoorOpen,
			}
		} else if RequestBelow(LocalOrderTable, currentFloor) {
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
		if RequestBelow(LocalOrderTable, currentFloor) {
			return DirnMovementPair{
				MotorDir: elevio.MD_Down,
				Movement: elev.EM_Moving,
			}
		} else if RequestHere(LocalOrderTable, currentFloor) {
			return DirnMovementPair{
				MotorDir: elevio.MD_Up,
				Movement: elev.EM_DoorOpen,
			}
		} else if RequestAbove(LocalOrderTable, currentFloor) {
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
		if RequestHere(LocalOrderTable, currentFloor) {
			return DirnMovementPair{
				MotorDir: elevio.MD_Stop,
				Movement: elev.EM_DoorOpen,
			}
		} else if RequestAbove(LocalOrderTable, currentFloor) {
			return DirnMovementPair{
				MotorDir: elevio.MD_Up,
				Movement: elev.EM_Moving,
			}
		} else if RequestBelow(LocalOrderTable, currentFloor) {
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

func ShouldStop(LocalOrderTable elev.LocalOrderTable, currentFloor int, dir elevio.MotorDirection) bool {
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

func ClearCurrentFloor(localOrderTable elev.LocalOrderTable,
	currentFloor int,
	dir elevio.MotorDirection) (elev.LocalOrderTable, [elev.N_BUTTONS]bool) {

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

func PrintLOT(localOrderTable elev.LocalOrderTable) {
	for floor := 0; floor < elev.N_FLOORS; floor++ {
		for btn := 0; btn < elev.N_BUTTONS; btn++ {
			if localOrderTable[floor][btn] {
				fmt.Printf("  #   ")
			} else {
				fmt.Printf("  -   ")
			}
		}
		fmt.Println()
	}
}
