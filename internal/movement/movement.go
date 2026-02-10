package movement

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/requests"
	"elevator-project-g7/internal/timer"
)

/*
- ElevatorPhysicalInfo
*/

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

	go elevio.PollObstructionSwitch(obstructionCh)
	go elevio.PollFloorSensor(floorArrivalCh)

	go HandleEvents(&elev, floorArrivalCh, obstructionCh)
}

func HandleEvents(elev *elev.ElevatorPhysicalInfo,
	floorArrivalCh chan int,
	obstructionCh chan bool) { //Her er forslag til struktur på FSM

	for {
		select {
		case <-elev.DoorTimer.C:
			FSM_OnDoorTimeout(elev)

		case floor := <-floorArrivalCh:
			FSM_OnFloorArrival(elev, floor)

		case obst := <-obstructionCh:
			elev.Obstructed = obst
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

}

func FSM_OnFloorArrival(e *elev.ElevatorPhysicalInfo, floor int) {

	elevio.SetFloorIndicator(floor)
	e.Floor = floor

	switch e.State {
	case elev.EM_Moving:
		if requests.ShouldStop(e.LocalOrderTable, e.Floor, e.MotorDir) {
			elevio.SetMotorDirection(elevio.MD_Stop)

			//Open door
			elevio.SetDoorOpenLamp(true)
			timer.Start(e.DoorTimer)
			e.State = elev.EM_DoorOpen

			//TODO: requests.clearShit //Implement clearShit in requests
			//TODO: C koden kaller setAllLights her? Skal vi??
		}
	}
}

func FSM_OnDoorTimeout(e *elev.ElevatorPhysicalInfo) {
	if e.Obstructed {
		timer.Start(e.DoorTimer)
		return
	}
	elevio.SetDoorOpenLamp(false)
	e.State = elev.EM_Idle
	timer.Stop(e.DoorTimer)
}
