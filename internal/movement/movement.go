package movement

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/requests"
	"fmt"
	"time"
)

func setAllLights(LocalOrderTable [elev.N_FLOORS][elev.N_BUTTONS]bool) {
	for f := 0; f < elev.N_FLOORS; f++ {
		for b := 0; b < elev.N_BUTTONS; b++ {
			elevio.SetButtonLamp(elevio.ButtonType(b), f, LocalOrderTable[f][b])
		}
	}
}

func Start(_id int, _port int) {
	e := elev.CreateElevator(_id, _port)

	//Input channels
	printTimerCh := make(chan bool)
	obstructionCh := make(chan bool)
	floorArrivalCh := make(chan int)
	buttonPressCh := make(chan elevio.ButtonEvent)

	go elevio.PollObstructionSwitch(obstructionCh)
	go elevio.PollFloorSensor(floorArrivalCh)
	go elevio.PollButtons(buttonPressCh)

	go generatePrintTimerEvents(printTimerCh)

	go HandleEvents(&e, floorArrivalCh, obstructionCh, buttonPressCh, printTimerCh)
}

func HandleEvents(e *elev.ElevatorPhysicalInfo,
	floorArrivalCh chan int,
	obstructionCh chan bool,
	buttonPressCh chan elevio.ButtonEvent,
	printTimerCh chan bool) { //Her er forslag til struktur på FSM

	for {
		select {

		case <-printTimerCh:
			fmt.Println("tenkte feil nvm")

		// TODO: eskil la til denne
		case btnEvent := <-buttonPressCh:
			btnFloor := btnEvent.Floor
			btnType := btnEvent.Button
			// Testing shit maybe unecesarry
			if !e.LocalOrderTable[btnFloor][btnType] {
				FSM_OnButtonPress(e, btnEvent)
			}
		case <-e.DoorTimer.C:
			fmt.Println("fsm doortimer event")
			FSM_OnDoorTimeout(e)

		case floor := <-floorArrivalCh:
			FSM_OnFloorArrival(e, floor)

		case obst := <-obstructionCh:
			e.Obstructed = obst
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

func FSM_OnButtonPress(e *elev.ElevatorPhysicalInfo, buttonEvent elevio.ButtonEvent) {
	btnType, btnFloor := buttonEvent.Button, buttonEvent.Floor

	switch e.State {

	case elev.EM_DoorOpen:
		if requests.ShouldClearImmediately(e.Floor, e.MotorDir, btnFloor, btnType) {
			// TODO: Marius; sjekk om jeg skjønte timeren riktig
			e.DoorTimer.Set(elev.DOOR_OPEN_TIME)
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
			// TODO: sjekk timer
			e.DoorTimer.Set(elev.DOOR_OPEN_TIME)
			e.DoorTimer.Start()
			fmt.Println("timer set 1")
			updated_LOT := requests.ClearCurrentFloor(e.LocalOrderTable, e.Floor, e.MotorDir)
			// TODO: MB logikk
			// We change the data here
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

func FSM_OnFloorArrival(e *elev.ElevatorPhysicalInfo, floor int) {

	e.Floor = floor
	elevio.SetFloorIndicator(floor)

	switch e.State {
	case elev.EM_Moving:
		if requests.ShouldStop(e.LocalOrderTable, e.Floor, e.MotorDir) {

			requests.PrintLOT(e.LocalOrderTable)

			elevio.SetMotorDirection(elevio.MD_Stop)
			elevio.SetDoorOpenLamp(true)
			updated_LOT := requests.ClearCurrentFloor(e.LocalOrderTable, e.Floor, e.MotorDir)
			// TODO: MB logikk
			e.LocalOrderTable = updated_LOT

			requests.PrintLOT(e.LocalOrderTable)

			e.DoorTimer.Set(elev.DOOR_OPEN_TIME)
			e.DoorTimer.Start()
			fmt.Println("timer set 3")

			//TODO: C koden kaller setAllLights her? Skal vi?? E: idk
			SetAllLights(e.LocalOrderTable)

			e.State = elev.EM_DoorOpen
		}
	}

	printElevatorState(e)
}

func FSM_OnDoorTimeout(e *elev.ElevatorPhysicalInfo) {

	/*
		if e.Obstructed {
			timer.Start(e.DoorTimer)
			return
			}
	*/

	switch e.State {

	case elev.EM_DoorOpen:
		pair := requests.ChooseDirection(e.LocalOrderTable, e.Floor, e.MotorDir)
		e.MotorDir = pair.Direction
		e.State = pair.Movement

		switch e.State {

		case elev.EM_DoorOpen:
			e.DoorTimer.Set(elev.DOOR_OPEN_TIME)
			e.DoorTimer.Start()
			fmt.Println("timer set 4")
			updated_LOT := requests.ClearCurrentFloor(e.LocalOrderTable, e.Floor, e.MotorDir)
			// TODO: MB logikk
			e.LocalOrderTable = updated_LOT
			setAllLights(e.LocalOrderTable)

		case elev.EM_Idle:
			elevio.SetDoorOpenLamp(false)
			elevio.SetMotorDirection(e.MotorDir)
			//elevio.SetMotorDirection(elevio.MD_Stop)

		// TOOD: This is maybe wrong ???
		// Neida va rett det, da funker heisen :=)
		case elev.EM_Moving:
			elevio.SetDoorOpenLamp(false)
			elevio.SetMotorDirection(e.MotorDir)

		default:
			fmt.Println("OnDoorTimeout case default")
		}
	}

	printElevatorState(e)
}

/*

	// TODO

	This was inside OnDoorTimeout

	elevio.SetDoorOpenLamp(false)
	e.State = elev.EM_Idle
	timer.Stop(e.DoorTimer)

*/

func printElevatorState(e *elev.ElevatorPhysicalInfo) {

	switch e.State {
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

func generatePrintTimerEvents(printTimerCh chan<- bool) {
	ticker := time.NewTicker(50000 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		printTimerCh <- true
	}

}
