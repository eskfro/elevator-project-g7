package movement

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
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

	// Local to Movement
	physicalInfo := initElev.PhysicalInfo
	prevLOT := initElev.PhysicalInfo.LocalOrderTable

	// Movement timers
	doorTimer := timer.New(elev.DOOR_OPEN_TIME)
	betweenFloorTimer := timer.New(elev.BETWEEN_FLOORS_TIMEOUT)
	defer doorTimer.Close()
	defer betweenFloorTimer.Close()

	// Marius timer
	// defer stuckTicker.Stop()
	// stuckTicker := time.NewTicker(elev.STUCK_TICKER_INTERVAL)
	// lastFloorChange := time.Now()
	// lastFloor := physicalInfo.Floor

	for {
		select {
		case newPhysicalInfo := <-updateMV_PhysicalInfo:
			log.Println("[Movement] PhysicalInfo Update")
			prevMovement := physicalInfo.Movement
			prevFloor := physicalInfo.Floor
			physicalInfo = newPhysicalInfo
			isLotChanged := physicalInfo.LocalOrderTable != prevLOT

			if isLotChanged {
				physicalInfo = fsm_OnTableUpdate(physicalInfo, doorTimer, fromMV_LOT, fromMV_Movement, fromMV_MotorDir, fromMV_ClearOrder)
			}

			// Sync and update
			syncBetweenFloorTimer(prevMovement, prevFloor, physicalInfo, betweenFloorTimer)
			prevLOT = physicalInfo.LocalOrderTable

		case <-doorTimer.C:
			log.Println("[Movement] Doortimer Event")
			prevMovement := physicalInfo.Movement
			prevFloor := physicalInfo.Floor

			physicalInfo = fsm_OnDoorTimeout(physicalInfo, doorTimer, fromMV_LOT, fromMV_Movement, fromMV_MotorDir, fromMV_ClearOrder)

			// Sync and update
			syncBetweenFloorTimer(prevMovement, prevFloor, physicalInfo, betweenFloorTimer)
			prevLOT = physicalInfo.LocalOrderTable

		case <-betweenFloorTimer.C:
			log.Fatalln("[Movement] Elevator stuck between floors!")

		// case <-stuckTicker.C:
		// 	hasActiveOrders := ordercontrol.HasOrders(physicalInfo.LocalOrderTable)
		// 	if hasActiveOrders && time.Since(lastFloorChange) > elev.STUCK_TIMEOUT {
		// 		log.Fatalln("[Movement] Elevator is stuck!")
		// 	}

		case newFloor := <-toMV_FloorArrival:
			fmt.Printf("[Movement]: Arrived at Floor = %d\n", newFloor)
			prevMovement := physicalInfo.Movement
			prevFloor := physicalInfo.Floor

			// Marius timer
			// if newFloor != lastFloor {
			// 	lastFloorChange = time.Now()
			// 	lastFloor = newFloor
			// }

			physicalInfo.Floor = newFloor
			physicalInfo = fsm_OnFloorArrival(physicalInfo, doorTimer, fromMV_LOT, fromMV_Movement, fromMV_ClearOrder)

			// Sync and update
			syncBetweenFloorTimer(prevMovement, prevFloor, physicalInfo, betweenFloorTimer)
			prevLOT = physicalInfo.LocalOrderTable

		}
	}
}

func syncBetweenFloorTimer(
	prevMovement elev.ElevatorMovement,
	prevFloor int,
	physicalInfo elev.ElevatorPhysicalInfo,
	betweenFloorTimer *timer.Timer,
) {
	enteredMoving := prevMovement != elev.EM_Moving &&
		physicalInfo.Movement == elev.EM_Moving

	leftMoving := prevMovement == elev.EM_Moving &&
		physicalInfo.Movement != elev.EM_Moving

	passedNewFloorWhileMoving := prevMovement == elev.EM_Moving &&
		physicalInfo.Movement == elev.EM_Moving &&
		physicalInfo.Floor != prevFloor

	switch {
	case leftMoving:
		betweenFloorTimer.Stop()

	case enteredMoving:
		betweenFloorTimer.Start()

	case passedNewFloorWhileMoving:
		betweenFloorTimer.Start() // Start() fungerer som reset
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
