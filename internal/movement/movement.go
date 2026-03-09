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

// [X]
func Movement(
	initElev elev.Elevator,
	ch_updateMV_PhysicalInfo chan elev.ElevatorPhysicalInfo,
	ch_fromMV_LOT chan elev.LocalOrderTable,
	ch_fromMV_Movement chan elev.ElevatorMovement,
	ch_fromMV_MotorDir chan elevio.MotorDirection,
	ch_fromMV_ClearOrder chan elev.ClearOrders,
	ch_toMV_FloorArrival chan int) {

	MV_PhysicalInfo := initElev.PhysicalInfo
	MV_prevLOT := initElev.PhysicalInfo.LocalOrderTable
	doorTimer := timer.New(elev.DOOR_OPEN_TIME)

	for {
		select {
		case newPhysicalInfo := <-ch_updateMV_PhysicalInfo:
			log.Println("[Movement] PhysicalInfo Update")
			MV_PhysicalInfo = newPhysicalInfo
			// TODO: maybe remove ts
			if MV_PhysicalInfo.LocalOrderTable == MV_prevLOT {
				continue
			}
			MV_PhysicalInfo = FSM_OnTableUpdate(MV_PhysicalInfo, doorTimer, ch_fromMV_LOT, ch_fromMV_Movement, ch_fromMV_MotorDir, ch_fromMV_ClearOrder)
			MV_prevLOT = MV_PhysicalInfo.LocalOrderTable

		case <-doorTimer.C:
			log.Println("[Movement] Doortimer Event")
			MV_PhysicalInfo = FSM_OnDoorTimeout(MV_PhysicalInfo, doorTimer, ch_fromMV_LOT, ch_fromMV_Movement, ch_fromMV_MotorDir, ch_fromMV_ClearOrder)
			MV_prevLOT = MV_PhysicalInfo.LocalOrderTable

		case newFloor := <-ch_toMV_FloorArrival:
			fmt.Printf("[Movement]: Arrived at Floor = %d\n", newFloor)
			MV_PhysicalInfo.Floor = newFloor
			MV_PhysicalInfo = FSM_OnFloorArrival(MV_PhysicalInfo, doorTimer, ch_fromMV_LOT, ch_fromMV_Movement, ch_fromMV_ClearOrder)
			MV_prevLOT = MV_PhysicalInfo.LocalOrderTable

		}
	}
}

func sendClearOrder(PhysicalInfo elev.ElevatorPhysicalInfo, buttonsToClear [elev.N_BUTTONS]bool, ch_fromMV_ClearOrder chan elev.ClearOrders) {
	var clearOrder elev.ClearOrders
	for btn := 0; btn < elev.N_BUTTONS; btn++ {
		if buttonsToClear[btn] {
			clearOrder[btn] = elev.Order{ElevId: PhysicalInfo.Id, Floor: PhysicalInfo.Floor, ButtonType: elevio.ButtonType(btn)}
		} else {
			clearOrder[btn] = elev.Order{ElevId: elev.INVALID_ELEVATOR_ID, Floor: PhysicalInfo.Floor, ButtonType: elevio.ButtonType(btn)}
		}
	}
	ch_fromMV_ClearOrder <- clearOrder
}

func FSM_OnTableUpdate(
	PhysicalInfo elev.ElevatorPhysicalInfo,
	doorTimer *timer.Timer,
	ch_fromMV_LOT chan elev.LocalOrderTable,
	ch_fromMV_Movement chan elev.ElevatorMovement,
	ch_fromMV_MotorDir chan elevio.MotorDirection,
	ch_fromMV_ClearOrder chan elev.ClearOrders) elev.ElevatorPhysicalInfo {

	switch PhysicalInfo.Movement {

	case elev.EM_DoorOpen:

		if requests.ShouldStop(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir) {

			doorTimer.Start()
			fmt.Println("timer set 2")
			updated_LOT, buttonsToClear := requests.ClearCurrentFloor(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir)
			PhysicalInfo.LocalOrderTable = updated_LOT
			PhysicalInfo.Movement = elev.EM_DoorOpen
			ch_fromMV_LOT <- PhysicalInfo.LocalOrderTable
			ch_fromMV_Movement <- PhysicalInfo.Movement

			sendClearOrder(PhysicalInfo, buttonsToClear, ch_fromMV_ClearOrder)
		}

		return PhysicalInfo

	case elev.EM_Moving:

	case elev.EM_Idle:

		pair := requests.ChooseDirection(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir)

		PhysicalInfo.MotorDir = pair.MotorDir
		PhysicalInfo.Movement = pair.Movement
		ch_fromMV_MotorDir <- PhysicalInfo.MotorDir
		ch_fromMV_Movement <- PhysicalInfo.Movement

		// Stay idle
		if PhysicalInfo.Movement == elev.EM_Idle {
			SetAllLights(PhysicalInfo.LocalOrderTable)
			return PhysicalInfo
		}

		// Switch on the newly chosen Movement-type
		switch PhysicalInfo.Movement {

		case elev.EM_DoorOpen:

			elevio.SetDoorOpenLamp(true)
			doorTimer.Start()
			fmt.Println("timer set 1")
			updated_LOT, buttonsToClear := requests.ClearCurrentFloor(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir)

			PhysicalInfo.LocalOrderTable = updated_LOT
			ch_fromMV_LOT <- PhysicalInfo.LocalOrderTable

			sendClearOrder(PhysicalInfo, buttonsToClear, ch_fromMV_ClearOrder)

		case elev.EM_Moving:

			elevio.SetDoorOpenLamp(false)
			elevio.SetMotorDirection(PhysicalInfo.MotorDir)

		default:

			fmt.Println("OnButtonPress case default")

		}

	}

	SetAllLights(PhysicalInfo.LocalOrderTable)
	printElevatorMovement(PhysicalInfo.Movement)
	return PhysicalInfo

}

func FSM_OnFloorArrival(
	PhysicalInfo elev.ElevatorPhysicalInfo,
	doorTimer *timer.Timer,
	ch_fromMV_LOT chan elev.LocalOrderTable,
	ch_fromMV_Movement chan elev.ElevatorMovement,
	ch_fromMV_ClearOrder chan elev.ClearOrders) elev.ElevatorPhysicalInfo {

	elevio.SetFloorIndicator(PhysicalInfo.Floor)

	if PhysicalInfo.Movement != elev.EM_Moving || !requests.ShouldStop(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir) {
		return PhysicalInfo
	}

	elevio.SetMotorDirection(elevio.MD_Stop)
	elevio.SetDoorOpenLamp(true)
	doorTimer.Start()
	fmt.Println("timer set 3")

	updated_LOT, buttonsToClear := requests.ClearCurrentFloor(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir)

	PhysicalInfo.LocalOrderTable = updated_LOT
	SetAllLights(updated_LOT)
	ch_fromMV_LOT <- updated_LOT

	sendClearOrder(PhysicalInfo, buttonsToClear, ch_fromMV_ClearOrder)

	PhysicalInfo.Movement = elev.EM_DoorOpen
	ch_fromMV_Movement <- elev.EM_DoorOpen

	return PhysicalInfo

}

func FSM_OnDoorTimeout(
	PhysicalInfo elev.ElevatorPhysicalInfo,
	doorTimer *timer.Timer,
	ch_fromMV_LOT chan elev.LocalOrderTable,
	ch_fromMV_Movement chan elev.ElevatorMovement,
	ch_fromMV_MotorDir chan elevio.MotorDirection,
	ch_fromMV_ClearOrder chan elev.ClearOrders) elev.ElevatorPhysicalInfo {

	if PhysicalInfo.Obstructed {

		doorTimer.Start()

		return PhysicalInfo
	}

	pair := requests.ChooseDirection(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir)
	PhysicalInfo.Movement = pair.Movement
	PhysicalInfo.MotorDir = pair.MotorDir
	ch_fromMV_Movement <- PhysicalInfo.Movement
	ch_fromMV_MotorDir <- PhysicalInfo.MotorDir

	switch PhysicalInfo.Movement {

	case elev.EM_DoorOpen:

		doorTimer.Start()
		fmt.Println("timer set 4")
		updated_LOT, buttonsToClear := requests.ClearCurrentFloor(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir)

		PhysicalInfo.LocalOrderTable = updated_LOT
		ch_fromMV_LOT <- PhysicalInfo.LocalOrderTable

		sendClearOrder(PhysicalInfo, buttonsToClear, ch_fromMV_ClearOrder)

		SetAllLights(updated_LOT)

	case elev.EM_Idle:

		elevio.SetDoorOpenLamp(false)
		elevio.SetMotorDirection(PhysicalInfo.MotorDir)

	case elev.EM_Moving:

		elevio.SetDoorOpenLamp(false)
		elevio.SetMotorDirection(PhysicalInfo.MotorDir)

	}

	printElevatorMovement(PhysicalInfo.Movement)
	return PhysicalInfo
}

// Make this elevio maybe
func SetAllLights(localOrderTable elev.LocalOrderTable) {
	for f := 0; f < elev.N_FLOORS; f++ {
		for b := 0; b < elev.N_BUTTONS; b++ {
			elevio.SetButtonLamp(elevio.ButtonType(b), f, localOrderTable[f][b])
		}
	}
}

// [X]
func printElevatorMovement(state elev.ElevatorMovement) {

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
