package elevio

import (
	"fmt"
	"strconv"
)

// Making channels for the inputs
func Inputs() (chan ButtonEvent, chan int, chan bool) {

	fromIO_BtnPress := make(chan ButtonEvent, 50)
	fromIO_Floor := make(chan int, 20)
	fromIO_Obstruction := make(chan bool, 20)

	go PollButtons(fromIO_BtnPress)
	go PollFloorSensor(fromIO_Floor)
	go PollObstructionSwitch(fromIO_Obstruction)

	return fromIO_BtnPress, fromIO_Floor, fromIO_Obstruction
}

func PrintStopButton(chanStopButton <-chan bool) {
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
