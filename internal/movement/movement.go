package movement

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/requests"
	"elevator-project-g7/internal/timer"
	"fmt"
	"log"
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

// [X]
func Movement(
	ch_Update chan elev.Elevator,
	ch_PrintTimer chan bool,
	ch_FloorArrival chan struct{},
	ch_LOTFromMV chan elev.LocalOrderTable,
	ch_StateFromMV chan elev.ElevatorMovement,
	ch_MotorDirFromMV chan elevio.MotorDirection) {

	var elevator elev.Elevator

	doorTimer := timer.New(elev.DOOR_OPEN_TIME)

	for {
		select {

		case elevator = <-ch_Update:

		case <-ch_PrintTimer:
			elev.PrintElevatorInfo(elevator)

		case <-doorTimer.C:
			fmt.Println("fsm doortimer event")
			FSM_OnDoorTimeout(elevator.PhysicalInfo, doorTimer, ch_LOTFromMV, ch_StateFromMV, ch_MotorDirFromMV)

		case <-ch_FloorArrival:

			FSM_OnFloorArrival(elevator.PhysicalInfo, doorTimer, ch_LOTFromMV, ch_StateFromMV)

		}
	}
}

// [X]
func SetAllLights(localOrderTable [elev.N_FLOORS][elev.N_BUTTONS]bool) {
	for f := 0; f < elev.N_FLOORS; f++ {
		for b := 0; b < elev.N_BUTTONS; b++ {
			elevio.SetButtonLamp(elevio.ButtonType(b), f, localOrderTable[f][b])
		}
	}
}

// [X]
func FSM_OnFloorArrival(
	pe elev.ElevatorPhysicalInfo,
	doorTimer *timer.Timer, // Maybe shit [X]
	ch_LOTFromMV chan elev.LocalOrderTable,
	ch_StateFromMV chan elev.ElevatorMovement) {

	elevio.SetFloorIndicator(pe.Floor)

	if pe.State != elev.EM_Moving || !requests.ShouldStop(pe.LocalOrderTable, pe.Floor, pe.MotorDir) {
		return
	}

	elevio.SetMotorDirection(elevio.MD_Stop)
	elevio.SetDoorOpenLamp(true)
	doorTimer.Start()
	fmt.Println("timer set 3")

	updated_LOT := requests.ClearCurrentFloor(pe.LocalOrderTable, pe.Floor, pe.MotorDir)
	ch_LOTFromMV <- updated_LOT

	SetAllLights(updated_LOT)

	ch_StateFromMV <- elev.EM_DoorOpen

}

// [X]
func FSM_OnDoorTimeout(pe elev.ElevatorPhysicalInfo,
	doorTimer *timer.Timer,
	ch_LOTFromMV chan elev.LocalOrderTable,
	ch_StateFromMV chan elev.ElevatorMovement,
	ch_MotorDirFromMV chan elevio.MotorDirection) {

	if pe.Obstructed {

		doorTimer.Start()

		return
	}

	switch pe.State {

	case elev.EM_DoorOpen:
		pair := requests.ChooseDirection(pe.LocalOrderTable, pe.Floor, pe.MotorDir)

		ch_StateFromMV <- pair.Movement
		ch_MotorDirFromMV <- pair.Direction

		switch pe.State {

		case elev.EM_DoorOpen:

			doorTimer.Start()

			fmt.Println("timer set 4")
			updated_LOT := requests.ClearCurrentFloor(pe.LocalOrderTable, pe.Floor, pe.MotorDir)
			ch_LOTFromMV <- updated_LOT

			setAllLights(updated_LOT)

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

// [X]
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

// TODO: slett før vi levera
func GeneratePrintTimerEvents(ch_printTimer chan<- bool) {
	ticker := time.NewTicker(2000 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		ch_printTimer <- true
	}

}

func AT_IsValidCombination(floor int, movement elev.ElevatorMovement, doorOpen bool) {

	movingWithOpenDoor := movement == elev.EM_Moving && doorOpen
	betweenFloorsWithOpenDoor := floor == -1 && doorOpen

	if movingWithOpenDoor || betweenFloorsWithOpenDoor {
		// TODO: bytt til log.Fatalln() når vi vet at systemete fungerer
		log.Println("AT_IsValidCombination triggered!")
		log.Printf("floor = %d | movement = %d | doorOpen = %t \n", floor, movement, doorOpen)

	}
}
