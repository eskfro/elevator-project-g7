package elevio

import (
	"fmt"
	"strconv"
)

func PrintStopButton(chanStopButton chan bool) {
	counter := 0
	for press := range chanStopButton {

		str := fmt.Sprintf("Someone pressed the Stop Button [%d] - (%t)", counter, press)
		fmt.Println(str)

		counter++
	}
}

func PrintButtonpress(buttonEvent ButtonEvent) {
	s_floor := strconv.Itoa(buttonEvent.Floor)
	switch buttonEvent.Button {
	case BT_Cab:
		fmt.Println("Cab button press at floor ", s_floor)
	case BT_HallDown:
		fmt.Println("Hall down button press at floor ", s_floor)
	case BT_HallUp:
		fmt.Println("Hall up button press at floor ", s_floor)
	}
}

// Moves elevator to a floor
func InitPhysicalElevator(ip string, port int, numFloors int) {

	Init(fmt.Sprintf("localhost:%d", port), numFloors)

	SetMotorDirection(MD_Down)
	for GetFloor() == -1 {
	}
	SetMotorDirection(MD_Stop)

	for GetObstruction() {
		SetDoorOpenLamp(true)
	}
	SetDoorOpenLamp(false)
	SetFloorIndicator(GetFloor())
}
