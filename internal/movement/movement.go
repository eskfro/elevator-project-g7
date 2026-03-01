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
	elevator elev.Elevator,
	ch_updateMV_Physicalnfo chan elev.ElevatorPhysicalInfo,
	ch_FloorArrival chan int,
	ch_fromMV_LOT chan elev.LocalOrderTable,
	ch_fromMV_State chan elev.ElevatorMovement,
	ch_fromMV_MotorDir chan elevio.MotorDirection) {

	MV_PhysicalInfo := elevator.PhysicalInfo

	doorTimer := timer.New(elev.DOOR_OPEN_TIME)

	for {
		select {

		case newPhysicalInfo := <-ch_updateMV_Physicalnfo:

			MV_PhysicalInfo = newPhysicalInfo
			MV_PhysicalInfo = FSM_OnTableUpdate(MV_PhysicalInfo, doorTimer, ch_fromMV_LOT, ch_fromMV_State, ch_fromMV_MotorDir)

		case <-doorTimer.C:

			fmt.Println("fsm doortimer event")
			MV_PhysicalInfo = FSM_OnDoorTimeout(MV_PhysicalInfo, doorTimer, ch_fromMV_LOT, ch_fromMV_State, ch_fromMV_MotorDir)

		case newFloor := <-ch_FloorArrival:

			MV_PhysicalInfo.Floor = newFloor
			MV_PhysicalInfo = FSM_OnFloorArrival(MV_PhysicalInfo, doorTimer, ch_fromMV_LOT, ch_fromMV_State)

		}
	}
}
func FSM_OnTableUpdate(PhysicalInfo elev.ElevatorPhysicalInfo,
	doorTimer *timer.Timer,
	ch_fromMV_LOT chan elev.LocalOrderTable,
	ch_fromMV_State chan elev.ElevatorMovement,
	ch_fromMV_MotorDir chan elevio.MotorDirection) elev.ElevatorPhysicalInfo {

	switch PhysicalInfo.State {

	case elev.EM_DoorOpen:
		if requests.ShouldStop(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir) {

			doorTimer.Start()
			fmt.Println("timer set 2")
			updated_LOT := requests.ClearCurrentFloor(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir)
			ch_fromMV_LOT <- updated_LOT
			PhysicalInfo.LocalOrderTable = updated_LOT
		}

	case elev.EM_Moving:
		// will be handled at floor when idle or something

	case elev.EM_Idle:

		pair := requests.ChooseDirection(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir)

		PhysicalInfo.MotorDir = pair.Direction
		PhysicalInfo.State = pair.Movement
		ch_fromMV_MotorDir <- pair.Direction
		ch_fromMV_State <- pair.Movement

		// Stay idle
		if pair.Movement == elev.EM_Idle {
			setAllLights(PhysicalInfo.LocalOrderTable)
			return PhysicalInfo
		}

		switch pair.Movement {

		case elev.EM_DoorOpen:
			elevio.SetDoorOpenLamp(true)
			doorTimer.Start()
			fmt.Println("timer set 1")

			updated_LOT := requests.ClearCurrentFloor(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir)
			PhysicalInfo.LocalOrderTable = updated_LOT
			ch_fromMV_LOT <- updated_LOT

		case elev.EM_Moving:
			elevio.SetDoorOpenLamp(false)
			elevio.SetMotorDirection(PhysicalInfo.MotorDir)

		default:
			fmt.Println("OnButtonPress case default")

		}

	}

	setAllLights(PhysicalInfo.LocalOrderTable)
	printElevatorState(PhysicalInfo.State)
	return PhysicalInfo

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
	PhysicalInfo elev.ElevatorPhysicalInfo,
	doorTimer *timer.Timer, // Maybe shit [X]
	ch_LOTFromMV chan elev.LocalOrderTable,
	ch_StateFromMV chan elev.ElevatorMovement) elev.ElevatorPhysicalInfo {

	elevio.SetFloorIndicator(PhysicalInfo.Floor)

	if PhysicalInfo.State != elev.EM_Moving || !requests.ShouldStop(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir) {
		return PhysicalInfo
	}

	elevio.SetMotorDirection(elevio.MD_Stop)
	elevio.SetDoorOpenLamp(true)
	doorTimer.Start()
	fmt.Println("timer set 3")

	updated_LOT := requests.ClearCurrentFloor(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir)
	PhysicalInfo.LocalOrderTable = updated_LOT
	ch_LOTFromMV <- updated_LOT

	SetAllLights(updated_LOT)

	PhysicalInfo.State = elev.EM_DoorOpen
	ch_StateFromMV <- elev.EM_DoorOpen

	return PhysicalInfo

}

// [X]
func FSM_OnDoorTimeout(PhysicalInfo elev.ElevatorPhysicalInfo,
	doorTimer *timer.Timer,
	ch_LOTFromMV chan elev.LocalOrderTable,
	ch_StateFromMV chan elev.ElevatorMovement,
	ch_MotorDirFromMV chan elevio.MotorDirection) elev.ElevatorPhysicalInfo {

	if PhysicalInfo.Obstructed {

		doorTimer.Start()

		return PhysicalInfo
	}

	switch PhysicalInfo.State {

	case elev.EM_DoorOpen:
		pair := requests.ChooseDirection(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir)

		PhysicalInfo.State = pair.Movement
		PhysicalInfo.MotorDir = pair.Direction
		ch_StateFromMV <- pair.Movement
		ch_MotorDirFromMV <- pair.Direction

		switch PhysicalInfo.State {

		case elev.EM_DoorOpen:

			doorTimer.Start()

			fmt.Println("timer set 4")
			updated_LOT := requests.ClearCurrentFloor(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir)
			PhysicalInfo.LocalOrderTable = updated_LOT
			ch_LOTFromMV <- updated_LOT

			setAllLights(updated_LOT)

		case elev.EM_Idle:
			elevio.SetDoorOpenLamp(false)
			elevio.SetMotorDirection(PhysicalInfo.MotorDir)

		case elev.EM_Moving:
			elevio.SetDoorOpenLamp(false)
			elevio.SetMotorDirection(PhysicalInfo.MotorDir)

		default:
			fmt.Println("OnDoorTimeout case default")

		}
	}

	printElevatorState(PhysicalInfo.State)
	return PhysicalInfo
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
		fmt.Printf("timer\n")
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
