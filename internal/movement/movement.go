package movement

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/requests"
	"fmt"
	"time"
)

type Channels struct {
	printTimer   chan bool
	obstruction  chan bool
	floorArrival chan int
}

func setAllLights(LocalOrderTable [elev.N_FLOORS][elev.N_BUTTONS]bool) {
	for f := 0; f < elev.N_FLOORS; f++ {
		for b := 0; b < elev.N_BUTTONS; b++ {
			elevio.SetButtonLamp(elevio.ButtonType(b), f, LocalOrderTable[f][b])
		}
	}
}

func Start(pe *elev.ElevatorPhysicalInfo,
	LocalOrderTable *[elev.N_FLOORS][elev.N_BUTTONS]bool) { //TODO: maybe pekar maybe not

	Ch := Channels{
		printTimer:   make(chan bool),
		obstruction:  make(chan bool),
		floorArrival: make(chan int),
	}

	go elevio.PollObstructionSwitch(Ch.obstruction)
	go elevio.PollFloorSensor(Ch.floorArrival)

	go generatePrintTimerEvents(Ch.printTimer)

	go HandleEvents(pe, LocalOrderTable, &Ch)
}

func HandleEvents(pe *elev.ElevatorPhysicalInfo,
	LocalOrderTable *[elev.N_FLOORS][elev.N_BUTTONS]bool,
	Ch *Channels) {

	for {
		select {

		case <-Ch.printTimer:
			fmt.Println(":) debugging melding :)")

		case <-pe.DoorTimer.C:
			fmt.Println("fsm doortimer event")
			FSM_OnDoorTimeout(pe, *LocalOrderTable)

		case floor := <-Ch.floorArrival:
			FSM_OnFloorArrival(pe, *LocalOrderTable, floor)

		case obst := <-Ch.obstruction:
			pe.Obstructed = obst
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

/*
func FSM_OnButtonPress(e *elev.ElevatorPhysicalInfo, buttonEvent elevio.ButtonEvent) {
	btnType, btnFloor := buttonEvent.Button, buttonEvent.Floor

	switch e.State {

	case elev.EM_DoorOpen:
		if requests.ShouldClearImmediately(e.Floor, e.MotorDir, btnFloor, btnType) {
			e.DoorTimer.Start()
			fmt.Println("timer set 2")
		} else {
			// TODO: her må det komme master/backup logic før det settes i local order table
			// Merker alle det gjelder med tag: "MB logikk"
			e.LocalOrderTable[btnFloor][btnType] = true
		}

	case elev.EM_Moving:
		// TODO: MB logikk
		e.LocalOrderTable[btnFloor][btnType] = true

	case elev.EM_Idle:
		// TODO: MB logikk
		e.LocalOrderTable[btnFloor][btnType] = true

		pair := requests.ChooseDirection(e.LocalOrderTable, e.Floor, e.MotorDir)
		e.MotorDir = pair.Direction
		e.State = pair.Movement

		switch pair.Movement {

		case elev.EM_DoorOpen:
			elevio.SetDoorOpenLamp(true)
			e.DoorTimer.Start()
			fmt.Println("timer set 1")
			updated_LOT := requests.ClearCurrentFloor(e.LocalOrderTable, e.Floor, e.MotorDir)
			// TODO: MB logikk
			e.LocalOrderTable = updated_LOT

		case elev.EM_Moving:
			elevio.SetMotorDirection(e.MotorDir)

		default:
			fmt.Println("OnButtonPress case default")
		}

	}

	// TODO: MB logikk
	setAllLights(e.LocalOrderTable)
	printElevatorState(e)

}
*/

func FSM_OnFloorArrival(pe *elev.ElevatorPhysicalInfo, LocalOrderTable [elev.N_FLOORS][elev.N_BUTTONS]bool, floor int) {

	pe.Floor = floor
	elevio.SetFloorIndicator(floor)

	switch pe.State {
	case elev.EM_Moving:
		if requests.ShouldStop(LocalOrderTable, pe.Floor, pe.MotorDir) {

			requests.PrintLOT(LocalOrderTable)

			elevio.SetMotorDirection(elevio.MD_Stop)
			elevio.SetDoorOpenLamp(true)
			updated_LOT := requests.ClearCurrentFloor(LocalOrderTable, pe.Floor, pe.MotorDir)
			// TODO: MB logikk
			LocalOrderTable = updated_LOT

			requests.PrintLOT(LocalOrderTable)
			pe.DoorTimer.Start()
			fmt.Println("timer set 3")

			SetAllLights(LocalOrderTable)

			pe.State = elev.EM_DoorOpen
		}
	}

	printElevatorState(pe.State)
}

func FSM_OnDoorTimeout(pe *elev.ElevatorPhysicalInfo, LocalOrderTable [elev.N_FLOORS][elev.N_BUTTONS]bool) {

	if pe.Obstructed {
		pe.DoorTimer.Start()
		return
	}

	switch pe.State {

	case elev.EM_DoorOpen:
		pair := requests.ChooseDirection(LocalOrderTable, pe.Floor, pe.MotorDir)
		pe.MotorDir = pair.Direction
		pe.State = pair.Movement

		switch pe.State {

		case elev.EM_DoorOpen:
			pe.DoorTimer.Start()
			fmt.Println("timer set 4")
			updated_LOT := requests.ClearCurrentFloor(LocalOrderTable, pe.Floor, pe.MotorDir)
			// TODO: MB logikk
			LocalOrderTable = updated_LOT
			setAllLights(LocalOrderTable)

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
func generatePrintTimerEvents(ch_printTimer chan<- bool) {
	ticker := time.NewTicker(50000 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		ch_printTimer <- true
	}

}
