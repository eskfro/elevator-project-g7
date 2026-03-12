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

	for f := currentFloor + 1; f < elev.N_FLOORS; f++ {
		for b := 0; b < elev.N_BUTTONS; b++ {
			if LocalOrderTable[f][b] {
				return true
			}
		}
	}
	return false
}

func RequestBelow(LocalOrderTable elev.LocalOrderTable, currentFloor int) bool {

	for f := 0; f < currentFloor; f++ {
		for b := 0; b < elev.N_BUTTONS; b++ {
			if LocalOrderTable[f][b] {
				return true
			}
		}
	}
	return false
}

func RequestHere(LocalOrderTable elev.LocalOrderTable, currentFloor int) bool {

	for b := 0; b < elev.N_BUTTONS; b++ {
		if LocalOrderTable[currentFloor][b] {
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

func ClearCurrentFloor(LocalOrderTable elev.LocalOrderTable,
	currentFloor int,
	dir elevio.MotorDirection) (elev.LocalOrderTable, [elev.N_BUTTONS]bool) {

	var buttonsToClear [elev.N_BUTTONS]bool

	LOT := LocalOrderTable
	cf := currentFloor

	// Clear the cab order
	LOT[cf][elevio.BT_Cab] = false
	buttonsToClear[elevio.BT_Cab] = true

	switch dir {

	case elevio.MD_Up:
		if !RequestAbove(LOT, cf) && !LOT[cf][elevio.BT_HallUp] {
			LOT[cf][elevio.BT_HallDown] = false
			buttonsToClear[elevio.BT_HallDown] = true
		}
		LOT[cf][elevio.BT_HallUp] = false
		buttonsToClear[elevio.BT_HallUp] = true

	case elevio.MD_Down:
		if !RequestBelow(LOT, cf) && !LOT[cf][elevio.BT_HallDown] {
			LOT[cf][elevio.BT_HallUp] = false
			buttonsToClear[elevio.BT_HallUp] = true

		}
		LOT[cf][elevio.BT_HallDown] = false
		buttonsToClear[elevio.BT_HallDown] = true

	default:
		LOT[cf][elevio.BT_HallUp] = false
		LOT[cf][elevio.BT_HallDown] = false
		buttonsToClear[elevio.BT_HallUp] = true
		buttonsToClear[elevio.BT_HallDown] = true

	}
	return LOT, buttonsToClear
}

func PrintLOT(LocalOrderTable elev.LocalOrderTable) {
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
