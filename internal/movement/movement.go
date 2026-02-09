package movement

import (
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/requests"
	"elevator-project-g7/internal/timer"
	"fmt"
	"time"
)

/*
- ElevatorPhysicalInfo
*/

const N_FLOORS = 4
const N_BUTTONS = 3
const DOOR_OPEN_TIME = time.Second * 3

type ElevatorMovement int

const (
	EM_Idle     ElevatorMovement = 0
	EM_Moving   ElevatorMovement = 1
	EM_DoorOpen ElevatorMovement = 2
)

type ElevatorPhysicalInfo struct {
	Id         int
	Port       int
	Floor      int
	MotorDir   elevio.MotorDirection
	State      ElevatorMovement
	NumFloors  int
	Obstructed bool
	DoorTimer  *timer.Timer
}

type DirnMovementPair struct {
	Direction elevio.MotorDirection
	Movement  ElevatorMovement
}

func setAllLights(e ElevatorPhysicalInfo, LocalOrderTable [N_FLOORS][N_BUTTONS]bool) {
	for f := 0; f < N_FLOORS; f++ {
		for b := 0; b < N_BUTTONS; b++ {
			elevio.SetButtonLamp(elevio.ButtonType(b), f, LocalOrderTable[f][b])
		}
	}
}

func Start(_id int, _port int) {
	elev := CreateElevator(_id, _port)

	//Input channels
	obstructionCh := make(chan bool)
	floorArrivalCh := make(chan int)

	go elevio.PollObstructionSwitch(obstructionCh)
	go elevio.PollFloorSensor(floorArrivalCh)

	go HandleEvents(&elev, floorArrivalCh, obstructionCh)
}

func HandleEvents(elev *ElevatorPhysicalInfo,
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

func SetAllLights(localOrderTable [N_FLOORS][N_BUTTONS]bool) {
	for f := 0; f < N_FLOORS; f++ {
		for b := 0; b < N_BUTTONS; b++ {
			elevio.SetButtonLamp(elevio.ButtonType(b), f, localOrderTable[f][b])
		}
	}
}

func CreateElevator(_Id int, _Port int) ElevatorPhysicalInfo {
	Elev := ElevatorPhysicalInfo{}
	Elev.Id = _Id
	Elev.Port = _Port
	Elev.NumFloors = N_FLOORS
	Elev.Floor = elevio.GetFloor()
	Elev.MotorDir = elevio.MD_Stop
	Elev.State = EM_Idle
	
	for f := 0; f < N_FLOORS; f++ {
		for b := 0; b < N_BUTTONS; b++ {
			Elev.LocalOrderTable[f][b] = false
		}
	}
	Elev.DoorTimer = timer.New(DOOR_OPEN_TIME)

	return Elev
}

func InitPhysicalElevator(ip string, port int, _initFloor int) {

	initFloor := _initFloor

	elevio.Init(fmt.Sprintf("localhost:%d", port), N_FLOORS)

	elevio.SetMotorDirection(elevio.MD_Down)
	for elevio.GetFloor() == -1 {
	}
	elevio.SetMotorDirection(elevio.MD_Stop)

	for elevio.GetObstruction() {
		elevio.SetDoorOpenLamp(true)
	}
	elevio.SetDoorOpenLamp(false)
	elevio.SetFloorIndicator(elevio.GetFloor())
}

func FSM_OnButtonPress(elev *ElevatorPhysicalInfo, buttonEvent elevio.ButtonEvent) {

}

func FSM_OnFloorArrival(elev *ElevatorPhysicalInfo, floor int) {

	elevio.SetFloorIndicator(floor)
	elev.Floor = floor

	switch elev.State {
	case EM_Moving:
		if requests.ShouldStop() { //TODO:Implement ShouldStop in requests
			elevio.SetMotorDirection(elevio.MD_Stop)

			//Open door
			elevio.SetDoorOpenLamp(true)
			if !DoorTimer.Stop() {
				select {case <- DoorTimer.C: default:}
			}
			DoorTimer.Reset(DOOR_OPEN_TIME)
			elev.State = EM_DoorOpen


			//TODO: requests.clearShit //Implement clearShit in requests
			//TODO: C koden kaller setAllLights her? Skal vi??
		}
	}
}

func FSM_OnDoorTimeout(elev *ElevatorPhysicalInfo){
	if elev.Obstructed{
		if !DoorTimer.Stop() {
				select {case <- DoorTimer.C: default:}
		}
		DoorTimer.Reset(DOOR_OPEN_TIME)	
		return
	}
	elevio.SetDoorOpenLamp(false)
	elev.State = EM_Idle
}
