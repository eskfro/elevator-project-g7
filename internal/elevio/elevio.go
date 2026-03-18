package elevio

import (
	"fmt"
	"strconv"
)

type MotorDirection int

const (
	MD_Up   MotorDirection = 1
	MD_Down MotorDirection = -1
	MD_Stop MotorDirection = 0
)

type ButtonType int

const (
	BT_HallUp   ButtonType = 0
	BT_HallDown ButtonType = 1
	BT_Cab      ButtonType = 2
)

type ButtonEvent struct {
	Floor  int
	Button ButtonType
}

func CreateHardwareChannels() (<-chan ButtonEvent, <-chan int, <-chan bool) {

	fromIO_BtnPress := make(chan ButtonEvent, 64)
	fromIO_Floor := make(chan int, 32)
	fromIO_Obstruction := make(chan bool, 32)

	go pollButtons(fromIO_BtnPress)
	go pollFloorSensor(fromIO_Floor)
	go pollObstructionSwitch(fromIO_Obstruction)

	return fromIO_BtnPress, fromIO_Floor, fromIO_Obstruction
}

func InitPhysicalElevator(ip string, port int, numFloors int) {

	Init(fmt.Sprintf("localhost:%d", port), numFloors)

	SetMotorDirection(MD_Down)
	for GetFloor() == -1 {
	}
	SetMotorDirection(MD_Stop)

	SetDoorOpenLamp(false)
	SetFloorIndicator(GetFloor())
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
