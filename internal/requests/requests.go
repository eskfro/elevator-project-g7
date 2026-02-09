package requests

import (
	"elevator-project-g7/internal/elevio"

	"golang.org/x/text/cases"
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

func ChooseDirection(LocalOrderTable [][]bool, currentFloor int, dir elevio.MotorDirection) {
	switch dir {
	case elevio.MD_Up:
		if RequestAbove(LocalOrderTable, currentFloor) {
			return ()
		}


	case elevio.MD_Down:
	case elevio.MD_Stop:

	}
}

