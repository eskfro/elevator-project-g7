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
	fromMV_ClearOrders chan<- elev.ClearOrders,
	toMV_FloorArrival <-chan int) {

	physicalInfo := initElev.PhysicalInfo

	doorTimer := timer.New(elev.DOOR_OPEN_TIME)
	betweenFloorTimer := timer.New(elev.BETWEEN_FLOORS_TIMEOUT)
	stuckDoorTimer := timer.New(elev.STUCK_DOOR_TIMEOUT)
	idleRestartTimer := timer.New(elev.IDLE_RESTART_TIMEOUT)
	defer doorTimer.Close()
	defer betweenFloorTimer.Close()
	defer stuckDoorTimer.Close()

	if physicalInfo.Movement == elev.EM_DoorOpen {
		stuckDoorTimer.Start()
	}
	if physicalInfo.Movement == elev.EM_Moving {
		betweenFloorTimer.Start()
	}
	if physicalInfo.Movement == elev.EM_DoorOpen && !doorTimer.IsRunning() {
		doorTimer.Start()
	}

	for {
		select {
		case <-betweenFloorTimer.C:
			log.Fatalf("[Movement] Elevator stuck between floors for %v!", elev.BETWEEN_FLOORS_TIMEOUT)

		case <-stuckDoorTimer.C:
			log.Fatalf("[Movement] The door was stuck open for %v!", elev.STUCK_DOOR_TIMEOUT)

		case <-idleRestartTimer.C:
			log.Fatalf("[Movement] Idle for %v, restart just in case", elev.IDLE_RESTART_TIMEOUT)

		// FSM onTableUpdate
		case newPhysicalInfo := <-updateMV_PhysicalInfo:
			isLotChanged := newPhysicalInfo.LocalOrderTable != physicalInfo.LocalOrderTable
			log.Println("[Movement] PhysicalInfo Update")
			prevMovement := physicalInfo.Movement
			prevFloor := physicalInfo.Floor
			physicalInfo = newPhysicalInfo

			if isLotChanged {
				physicalInfo = fsm_onTableUpdate(physicalInfo, doorTimer, fromMV_LOT, fromMV_Movement, fromMV_MotorDir, fromMV_ClearOrders)
			}

			syncBetweenFloorTimer(prevMovement, prevFloor, physicalInfo, betweenFloorTimer)
			syncStuckDoorTimer(physicalInfo, stuckDoorTimer)
			syncIdleRestartTimer(physicalInfo, idleRestartTimer)

		// FSM onDoorTimeout
		case <-doorTimer.C:
			log.Println("[Movement] Doortimer Event")
			prevMovement := physicalInfo.Movement
			prevFloor := physicalInfo.Floor

			physicalInfo = fsm_onDoorTimeout(physicalInfo, doorTimer, fromMV_LOT, fromMV_Movement, fromMV_MotorDir, fromMV_ClearOrders)

			syncBetweenFloorTimer(prevMovement, prevFloor, physicalInfo, betweenFloorTimer)
			syncStuckDoorTimer(physicalInfo, stuckDoorTimer)
			syncIdleRestartTimer(physicalInfo, idleRestartTimer)

		// FSM onFloorArrival
		case newFloor := <-toMV_FloorArrival:
			fmt.Printf("[Movement]: Arrived at Floor = %d\n", newFloor)
			prevMovement := physicalInfo.Movement
			prevFloor := physicalInfo.Floor
			if newFloor == 0 || newFloor == (elev.N_FLOORS-1) {
				elevio.SetMotorDirection(elevio.MD_Stop)
			}
			physicalInfo.Floor = newFloor

			physicalInfo = fsm_onFloorArrival(physicalInfo, doorTimer, fromMV_LOT, fromMV_Movement, fromMV_ClearOrders)

			syncBetweenFloorTimer(prevMovement, prevFloor, physicalInfo, betweenFloorTimer)
			syncStuckDoorTimer(physicalInfo, stuckDoorTimer)
			syncIdleRestartTimer(physicalInfo, idleRestartTimer)

		}
	}
}

func syncIdleRestartTimer(
	physicalInfo elev.ElevatorPhysicalInfo,
	idleRestartTimer *timer.Timer,
) {
	if idleRestartTimer == nil {
		return
	}

	isIdle := physicalInfo.Movement == elev.EM_Idle

	if !isIdle {
		idleRestartTimer.Stop()
		return
	}

	if !idleRestartTimer.IsRunning() {
		idleRestartTimer.Start()
	}

}

func syncStuckDoorTimer(
	physicalInfo elev.ElevatorPhysicalInfo,
	stuckDoorTimer *timer.Timer,
) {
	if stuckDoorTimer == nil {
		return
	}

	isDoorOpen := physicalInfo.Movement == elev.EM_DoorOpen

	if !isDoorOpen {
		stuckDoorTimer.Stop()
		return
	}

	if !stuckDoorTimer.IsRunning() {
		stuckDoorTimer.Start()
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
		betweenFloorTimer.Start()
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
			clearOrder[btn] = elev.Order{ElevID: PhysicalInfo.ID, Floor: PhysicalInfo.Floor, ButtonType: elevio.ButtonType(btn)}
		} else {
			clearOrder[btn] = elev.Order{ElevID: elev.INVALID_ELEVATOR_ID, Floor: PhysicalInfo.Floor, ButtonType: elevio.ButtonType(btn)}
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
