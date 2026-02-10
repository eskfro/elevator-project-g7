package movement

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/requests"
	"elevator-project-g7/internal/timer"
)

func setAllLights(LocalOrderTable [elev.N_FLOORS][elev.N_BUTTONS]bool) {
	for f := 0; f < elev.N_FLOORS; f++ {
		for b := 0; b < elev.N_BUTTONS; b++ {
			elevio.SetButtonLamp(elevio.ButtonType(b), f, LocalOrderTable[f][b])
		}
	}
}

func Start(_id int, _port int) {
	elev := elev.CreateElevator(_id, _port)

	//Input channels
	obstructionCh := make(chan bool)
	floorArrivalCh := make(chan int)
	buttonPressCh := make(chan elevio.ButtonEvent)

	go elevio.PollObstructionSwitch(obstructionCh)
	go elevio.PollFloorSensor(floorArrivalCh)
	go elevio.PollButtons(buttonPressCh)

	go HandleEvents(&elev, floorArrivalCh, obstructionCh, buttonPressCh)
}

func HandleEvents(e *elev.ElevatorPhysicalInfo,
	floorArrivalCh chan int,
	obstructionCh chan bool,
	buttonPressCh chan elevio.ButtonEvent) { //Her er forslag til struktur på FSM

	for {
		select {

		// TODO: eskil la til denne
		case btnEvent := <-buttonPressCh:
			FSM_OnButtonPress(e, btnEvent)

		case <-e.DoorTimer.C:
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
			timer.Start(e.DoorTimer)
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
			timer.Start(e.DoorTimer)
			updated_LOT := requests.ClearCurrentFloor(e.LocalOrderTable, e.Floor, e.MotorDir)
			// TODO: MB logikk
			// We change the data here
			e.LocalOrderTable = updated_LOT

		case elev.EM_Moving:
			elevio.SetMotorDirection(e.MotorDir)

		}

	}

	// TODO: MB logikk
	setAllLights(e.LocalOrderTable)

}

func FSM_OnFloorArrival(e *elev.ElevatorPhysicalInfo, floor int) {

	e.Floor = floor
	elevio.SetFloorIndicator(floor)

	switch e.State {
	case elev.EM_Moving:
		if requests.ShouldStop(e.LocalOrderTable, e.Floor, e.MotorDir) {
			elevio.SetMotorDirection(elevio.MD_Stop)
			elevio.SetDoorOpenLamp(true)
			updated_LOT := requests.ClearCurrentFloor(e.LocalOrderTable, e.Floor, e.MotorDir)
			// TODO: MB logikk
			e.LocalOrderTable = updated_LOT

			timer.Start(e.DoorTimer)

			//TODO: C koden kaller setAllLights her? Skal vi?? E: idk
			SetAllLights(e.LocalOrderTable)

			e.State = elev.EM_DoorOpen
		}
	}
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
			timer.Start(e.DoorTimer)
			updated_LOT := requests.ClearCurrentFloor(e.LocalOrderTable, e.Floor, e.MotorDir)
			// TODO: MB logikk
			e.LocalOrderTable = updated_LOT
			setAllLights(e.LocalOrderTable)
		case elev.EM_Idle:
			elevio.SetDoorOpenLamp(false)
			elevio.SetMotorDirection(e.MotorDir)
		}
	}
}

/*

	// TODO

	This was inside OnDoorTimeout

	elevio.SetDoorOpenLamp(false)
	e.State = elev.EM_Idle
	timer.Stop(e.DoorTimer)

*/
