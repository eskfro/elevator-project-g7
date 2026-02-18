package movement

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/requests"
	"elevator-project-g7/internal/timer"
	"fmt"
	"time"
)

type Channels struct {
	PrintTimer   chan bool
	Obstruction  chan bool
	FloorArrival chan int
}

func setAllLights(LocalOrderTable [elev.N_FLOORS][elev.N_BUTTONS]bool) {
	for f := 0; f < elev.N_FLOORS; f++ {
		for b := 0; b < elev.N_BUTTONS; b++ {
			elevio.SetButtonLamp(elevio.ButtonType(b), f, LocalOrderTable[f][b])
		}
	}
}

/*
func Start(pe *elev.ElevatorPhysicalInfo,
	LocalOrderTable *[elev.N_FLOORS][elev.N_BUTTONS]bool) { //TODO: maybe pekar maybe not

	Ch := Channels{
		printTimer:   make(chan bool),
		obstruction:  make(chan bool),
		floorArrival: make(chan int),
	}

	go elevio.PollObstructionSwitch(Ch.obstruction)
	go elevio.PollFloorSensor(Ch.floorArrival)

	go generatePrintTimerEvent
				elevator.AllOrderTables[senderID] = rcvOrs(Ch.printTimer)

	go HandleEvents(pe, LocalOrderTable, &Ch)
}
*/

func Movement(
	ch_Update chan elev.Elevator,
	ch_PrintTimer chan bool,
	ch_FloorArrival chan int) {

	var elevator elev.Elevator
	doorTimer := timer.New(elev.DOOR_OPEN_TIME)
	for {
		select {

		case elevator = <-ch_Update:

		case <-ch_PrintTimer:
			fmt.Println(":) debugging melding :)")

		case <-doorTimer.C:
			fmt.Println("fsm doortimer event")
			FSM_OnDoorTimeout(elevator, doorTimer)

		case floor := <-ch_FloorArrival:
			FSM_OnFloorArrival(elevator, floor)

		}
	}
}

func SetAllLights(localOrderTable [elev.N_FLOORS][elev.N_BUTTONS]bool) {
	for f := 0; f < elev.N_FLOORS; f++ {
		for b := 0; b < elev.N_BUTTONS; b++ {
			elevio.SetButtonLamp(elevio.ButtonType(b), f, localOrderTable[f][b])
		}
	}
}

func FSM_OnFloorArrival(elevator elev.Elevator,
	doorTimer *timer.Timer,
	floor int) {

	pe := elevator.PhysicalInfo

	pe.Floor = floor
	elevio.SetFloorIndicator(floor)

	switch pe.State {
	case elev.EM_Moving:
		if requests.ShouldStop(pe.LocalOrderTable, pe.Floor, pe.MotorDir) {

			requests.PrintLOT(pe.LocalOrderTable)

			elevio.SetMotorDirection(elevio.MD_Stop)
			elevio.SetDoorOpenLamp(true)

			updated_LOT := requests.ClearCurrentFloor(pe.LocalOrderTable, pe.Floor, pe.MotorDir)
			// TODO: MB logikk
			pe.LocalOrderTable = updated_LOT

			requests.PrintLOT(pe.LocalOrderTable)
			doorTimer.Start()
			fmt.Println("timer set 3")

			SetAllLights(pe.LocalOrderTable)

			// TODO: Update shit
			pe.State = elev.EM_DoorOpen
		}
	}

	printElevatorState(pe.State)
}

func FSM_OnDoorTimeout(elevator elev.Elevator, doorTimer *timer.Timer) {

	pe := elevator.PhysicalInfo

	if pe.Obstructed {

		doorTimer.Start()

		return
	}

	switch pe.State {

	case elev.EM_DoorOpen:
		pair := requests.ChooseDirection(pe.LocalOrderTable, pe.Floor, pe.MotorDir)
		pe.MotorDir = pair.Direction
		pe.State = pair.Movement

		switch pe.State {

		case elev.EM_DoorOpen:
			pe.DoorTimer.Start()
			fmt.Println("timer set 4")
			updated_LOT := requests.ClearCurrentFloor(pe.LocalOrderTable, pe.Floor, pe.MotorDir)
			// TODO: MB logikk
			pe.LocalOrderTable = updated_LOT
			setAllLights(pe.LocalOrderTable)

		case elev.EM_Idle:
			elevio.SetDoorOpenLamp(false)
			elevio.SetMotorDirection(pe.MotorDir)

		case elev.EM_Moving:
			elevio.SetDoorOpenLamp(false)
			elevio.SetMotorDirection(pe.MotorDir)

		default:
			fmt.Println("OnDoorTimeout case default")
		}
	}

	printElevatorState(pe.State)
}

func printElevatorState(state elev.ElevatorMovement) {

	switch state {
	case elev.EM_Idle:
		fmt.Println("state = IDLE")
	case elev.EM_Moving:
		fmt.Println("state = MOVING")
	case elev.EM_DoorOpen:
		fmt.Println("state = DOOR OPEN")
	default:
		fmt.Println("state = ?")
	}
}

// Maybe shit
func GeneratePrintTimerEvents(ch_printTimer chan<- bool) {
	ticker := time.NewTicker(50000 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		ch_printTimer <- true
	}

}
