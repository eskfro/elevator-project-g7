package movement

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/requests"
	"elevator-project-g7/internal/timer"
	"fmt"
	"log"
)

func Movement(
	initElev elev.Elevator,
	updateMV_PhysicalInfo <-chan elev.ElevatorPhysicalInfo,
	fromMV_LOT chan<- elev.LocalOrderTable,
	fromMV_Movement chan<- elev.ElevatorMovement,
	fromMV_MotorDir chan<- elevio.MotorDirection,
	fromMV_ClearOrder chan<- elev.ClearOrders,
	toMV_FloorArrival <-chan int) {

	physicalInfo := initElev.PhysicalInfo
	prevLOT := initElev.PhysicalInfo.LocalOrderTable
	doorTimer := timer.New(elev.DOOR_OPEN_TIME)

	for {
		select {
		case newPhysicalInfo := <-updateMV_PhysicalInfo:
			log.Println("[Movement] PhysicalInfo Update")
			physicalInfo = newPhysicalInfo

			isChanged := physicalInfo.LocalOrderTable == prevLOT
			prevLOT = physicalInfo.LocalOrderTable

			if isChanged || physicalInfo.Movement == elev.EM_Idle {
				physicalInfo = fsm_OnTableUpdate(physicalInfo, doorTimer, fromMV_LOT, fromMV_Movement, fromMV_MotorDir, fromMV_ClearOrder)
			}

		case <-doorTimer.C:
			log.Println("[Movement] Doortimer Event")
			physicalInfo = fsm_OnDoorTimeout(physicalInfo, doorTimer, fromMV_LOT, fromMV_Movement, fromMV_MotorDir, fromMV_ClearOrder)
			prevLOT = physicalInfo.LocalOrderTable

		case newFloor := <-toMV_FloorArrival:
			fmt.Printf("[Movement]: Arrived at Floor = %d\n", newFloor)
			physicalInfo.Floor = newFloor
			physicalInfo = fsm_OnFloorArrival(physicalInfo, doorTimer, fromMV_LOT, fromMV_Movement, fromMV_ClearOrder)
			prevLOT = physicalInfo.LocalOrderTable

		}
	}
}

func anyOrderToClear(buttonsToClear [elev.N_BUTTONS]bool) bool {
	for btn := 0; btn < elev.N_BUTTONS; btn++ {
		if buttonsToClear[btn] == true {
			return true
		}
	}
	return false
}

func sendClearOrder(PhysicalInfo elev.ElevatorPhysicalInfo, buttonsToClear [elev.N_BUTTONS]bool, fromMV_ClearOrder chan<- elev.ClearOrders) {
	var clearOrder elev.ClearOrders
	for btn := 0; btn < elev.N_BUTTONS; btn++ {
		if buttonsToClear[btn] {
			clearOrder[btn] = elev.Order{ElevId: PhysicalInfo.Id, Floor: PhysicalInfo.Floor, ButtonType: elevio.ButtonType(btn)}
		} else {
			clearOrder[btn] = elev.Order{ElevId: elev.INVALID_ELEVATOR_ID, Floor: PhysicalInfo.Floor, ButtonType: elevio.ButtonType(btn)}
		}
	}
	fromMV_ClearOrder <- clearOrder
}

func fsm_OnTableUpdate(
	PhysicalInfo elev.ElevatorPhysicalInfo,
	doorTimer *timer.Timer,
	fromMV_LOT chan<- elev.LocalOrderTable,
	fromMV_Movement chan<- elev.ElevatorMovement,
	fromMV_MotorDir chan<- elevio.MotorDirection,
	fromMV_ClearOrder chan<- elev.ClearOrders,
) elev.ElevatorPhysicalInfo {

	prevMotorDir := PhysicalInfo.MotorDir
	prevMovement := PhysicalInfo.Movement
	prevLOT := PhysicalInfo.LocalOrderTable

	switch PhysicalInfo.Movement {

	case elev.EM_DoorOpen:

		if requests.ShouldStop(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir) {
			// Stay DoorOpen and reset door timer
			PhysicalInfo.Movement = elev.EM_DoorOpen
			doorTimer.Start()
			fmt.Println("timer set 2")

			updated_LOT, buttonsToClear := requests.ClearCurrentFloor(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir)
			PhysicalInfo.LocalOrderTable = updated_LOT

			// Updates from MV
			if anyOrderToClear(buttonsToClear) {
				sendClearOrder(PhysicalInfo, buttonsToClear, fromMV_ClearOrder)
			}
			// Guard Added 11.03
			if PhysicalInfo.LocalOrderTable != prevLOT {
				fromMV_LOT <- PhysicalInfo.LocalOrderTable
			}
			if PhysicalInfo.Movement != prevMovement {
				fromMV_Movement <- PhysicalInfo.Movement
			}
		}

	case elev.EM_Moving:

	case elev.EM_Idle:
		pair := requests.ChooseDirection(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir)
		PhysicalInfo.MotorDir = pair.MotorDir
		PhysicalInfo.Movement = pair.Movement

		if PhysicalInfo.MotorDir != prevMotorDir {
			fromMV_MotorDir <- PhysicalInfo.MotorDir
		}
		if PhysicalInfo.Movement != prevMovement {
			fromMV_Movement <- PhysicalInfo.Movement
		}

		// Stay idle
		if PhysicalInfo.Movement == elev.EM_Idle {
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

			// Updates from MV
			if anyOrderToClear(buttonsToClear) {
				sendClearOrder(PhysicalInfo, buttonsToClear, fromMV_ClearOrder)
			}
			if PhysicalInfo.LocalOrderTable != prevLOT {
				fromMV_LOT <- PhysicalInfo.LocalOrderTable
			}

		case elev.EM_Moving:
			elevio.SetDoorOpenLamp(false)
			elevio.SetMotorDirection(PhysicalInfo.MotorDir)

		default:
			fmt.Println("OnButtonPress case default")

		}

	}

	printElevatorMovement(PhysicalInfo.Movement)
	return PhysicalInfo

}

func fsm_OnFloorArrival(
	PhysicalInfo elev.ElevatorPhysicalInfo,
	doorTimer *timer.Timer,
	fromMV_LOT chan<- elev.LocalOrderTable,
	fromMV_Movement chan<- elev.ElevatorMovement,
	fromMV_ClearOrder chan<- elev.ClearOrders,
) elev.ElevatorPhysicalInfo {

	prevMovement := PhysicalInfo.Movement
	prevLOT := PhysicalInfo.LocalOrderTable

	elevio.SetFloorIndicator(PhysicalInfo.Floor)

	isMoving := PhysicalInfo.Movement == elev.EM_Moving
	shouldStop := requests.ShouldStop(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir)

	if !isMoving || !shouldStop {
		return PhysicalInfo
	}

	elevio.SetMotorDirection(elevio.MD_Stop)
	elevio.SetDoorOpenLamp(true)
	PhysicalInfo.Movement = elev.EM_DoorOpen
	doorTimer.Start()
	fmt.Println("timer set 3")

	// Clear current floor
	updated_LOT, buttonsToClear := requests.ClearCurrentFloor(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir)
	PhysicalInfo.LocalOrderTable = updated_LOT

	// Updates from MV
	if anyOrderToClear(buttonsToClear) {
		sendClearOrder(PhysicalInfo, buttonsToClear, fromMV_ClearOrder)
	}

	if PhysicalInfo.LocalOrderTable != prevLOT {
		fromMV_LOT <- PhysicalInfo.LocalOrderTable
	}
	if PhysicalInfo.Movement != prevMovement {
		fromMV_Movement <- PhysicalInfo.Movement
	}

	printElevatorMovement(PhysicalInfo.Movement)
	return PhysicalInfo

}

func fsm_OnDoorTimeout(
	PhysicalInfo elev.ElevatorPhysicalInfo,
	doorTimer *timer.Timer,
	fromMV_LOT chan<- elev.LocalOrderTable,
	fromMV_Movement chan<- elev.ElevatorMovement,
	fromMV_MotorDir chan<- elevio.MotorDirection,
	fromMV_ClearOrder chan<- elev.ClearOrders,
) elev.ElevatorPhysicalInfo {

	prevMovement := PhysicalInfo.Movement
	prevLOT := PhysicalInfo.LocalOrderTable
	isObstructed := PhysicalInfo.Obstructed

	if isObstructed {
		doorTimer.Start()
		return PhysicalInfo
	}

	// Chose direction
	pair := requests.ChooseDirection(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir)
	PhysicalInfo.Movement = pair.Movement
	PhysicalInfo.MotorDir = pair.MotorDir
	fromMV_MotorDir <- PhysicalInfo.MotorDir

	if PhysicalInfo.Movement != prevMovement {
		fromMV_Movement <- PhysicalInfo.Movement
	}

	switch PhysicalInfo.Movement {

	case elev.EM_DoorOpen:
		doorTimer.Start()
		fmt.Println("timer set 4")

		// Clear current floor
		updated_LOT, buttonsToClear := requests.ClearCurrentFloor(PhysicalInfo.LocalOrderTable, PhysicalInfo.Floor, PhysicalInfo.MotorDir)
		PhysicalInfo.LocalOrderTable = updated_LOT

		// Updates from MV
		if anyOrderToClear(buttonsToClear) {
			sendClearOrder(PhysicalInfo, buttonsToClear, fromMV_ClearOrder)
		}
		if PhysicalInfo.LocalOrderTable != prevLOT {
			fromMV_LOT <- PhysicalInfo.LocalOrderTable
		}

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

// TODO: Lag slike funksjoner for fault tolerance kanskjer. Men vi setter aldri floor til (-1) da.
func AT_IsValidCombination(floor int, movement elev.ElevatorMovement, doorOpen bool) {

	isMoving := movement == elev.EM_Moving

	if doorOpen && isMoving {
		log.Printf("floor = %d | movement = %d | doorOpen = %t \n", floor, movement, doorOpen)
		log.Fatalln("AT_IsValidCombination triggered: Moving with door open!")
	}
}
