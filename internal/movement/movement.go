package movement

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/requests"
	"elevator-project-g7/internal/timer"
	"fmt"
	"log"
)

// [X]
func Movement(
	initElev elev.Elevator,
	updateMV_PhysicalInfo <-chan elev.ElevatorPhysicalInfo,
	updateMV_OrderTable <-chan elev.OrderTable,
	updateMV_AliveList <-chan elev.AliveList,
	fromMV_LOT chan<- elev.LocalOrderTable,
	fromMV_Movement chan<- elev.ElevatorMovement,
	fromMV_MotorDir chan<- elevio.MotorDirection,
	fromMV_ClearOrder chan<- elev.ClearOrders,
	toMV_FloorArrival <-chan int) {

	physicalInfo := initElev.PhysicalInfo
	orderTable := initElev.OrderTable
	aliveList := initElev.AliveList
	prevLOT := initElev.PhysicalInfo.LocalOrderTable
	doorTimer := timer.New(elev.DOOR_OPEN_TIME)

	for {
		select {
		case newPhysicalInfo := <-updateMV_PhysicalInfo:
			log.Println("[Movement] PhysicalInfo Update")
			physicalInfo = newPhysicalInfo

			if physicalInfo.LocalOrderTable == prevLOT {
				continue
			}

			physicalInfo = FSM_OnTableUpdate(physicalInfo, orderTable, aliveList, doorTimer, fromMV_LOT, fromMV_Movement, fromMV_MotorDir, fromMV_ClearOrder)
			prevLOT = physicalInfo.LocalOrderTable

		case orderTable = <-updateMV_OrderTable:
			SetAllLights(physicalInfo.LocalOrderTable, orderTable, aliveList)

		case aliveList = <-updateMV_AliveList:
			SetAllLights(physicalInfo.LocalOrderTable, orderTable, aliveList)

		case <-doorTimer.C:
			log.Println("[Movement] Doortimer Event")
			physicalInfo = FSM_OnDoorTimeout(physicalInfo, orderTable, aliveList, doorTimer, fromMV_LOT, fromMV_Movement, fromMV_MotorDir, fromMV_ClearOrder)
			prevLOT = physicalInfo.LocalOrderTable

		case newFloor := <-toMV_FloorArrival:
			fmt.Printf("[Movement]: Arrived at Floor = %d\n", newFloor)
			physicalInfo.Floor = newFloor
			physicalInfo = FSM_OnFloorArrival(physicalInfo, orderTable, aliveList, doorTimer, fromMV_LOT, fromMV_Movement, fromMV_ClearOrder)
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

func FSM_OnTableUpdate(
	PhysicalInfo elev.ElevatorPhysicalInfo,
	orderTable elev.OrderTable,
	aliveList elev.AliveList,
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
			SetAllLights(PhysicalInfo.LocalOrderTable, orderTable, aliveList)

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
		SetAllLights(PhysicalInfo.LocalOrderTable, orderTable, aliveList)

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
			SetAllLights(PhysicalInfo.LocalOrderTable, orderTable, aliveList)
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
			SetAllLights(PhysicalInfo.LocalOrderTable, orderTable, aliveList)

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

	SetAllLights(PhysicalInfo.LocalOrderTable, orderTable, aliveList)
	printElevatorMovement(PhysicalInfo.Movement)
	return PhysicalInfo

}

func FSM_OnFloorArrival(
	PhysicalInfo elev.ElevatorPhysicalInfo,
	orderTable elev.OrderTable,
	aliveList elev.AliveList,
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
	SetAllLights(PhysicalInfo.LocalOrderTable, maskedOrderTable(orderTable, PhysicalInfo.Floor, buttonsToClear), aliveList)

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

	SetAllLights(PhysicalInfo.LocalOrderTable, orderTable, aliveList)
	printElevatorMovement(PhysicalInfo.Movement)
	return PhysicalInfo

}

func FSM_OnDoorTimeout(
	PhysicalInfo elev.ElevatorPhysicalInfo,
	orderTable elev.OrderTable,
	aliveList elev.AliveList,
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
		SetAllLights(PhysicalInfo.LocalOrderTable, maskedOrderTable(orderTable, PhysicalInfo.Floor, buttonsToClear), aliveList)

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

	SetAllLights(PhysicalInfo.LocalOrderTable, orderTable, aliveList)
	printElevatorMovement(PhysicalInfo.Movement)
	return PhysicalInfo
}

func maskedOrderTable(orderTable elev.OrderTable, floor int, buttonsToClear [elev.N_BUTTONS]bool) elev.OrderTable {
	maskedOT := orderTable
	for id := range maskedOT {
		for btn := 0; btn < elev.N_BUTTONS; btn++ {
			if buttonsToClear[btn] {
				maskedOT[id][floor][btn] = elev.OS_NO_ORDER
			}
		}
	}
	return maskedOT
}

// Føkka opp import-greier når eg flytta til elevio
func SetAllLights(localOrderTable elev.LocalOrderTable, orderTable elev.OrderTable, aliveList elev.AliveList) {
	for floor := 0; floor < elev.N_FLOORS; floor++ {
		for btn := 0; btn < elev.N_BUTTONS; btn++ {
			shouldLightUp := false
			btnType := elevio.ButtonType(btn)

			if btnType == elevio.BT_Cab {
				if localOrderTable[floor][btn] {
					shouldLightUp = true
				}
			} else {
				for id, phys := range aliveList {
					if phys.Role == elev.ER_Dead {
						continue
					}
					if orderTable[id][floor][btn] == elev.OS_CONFIRMED {
						shouldLightUp = true
						break
					}
				}
			}

			elevio.SetButtonLamp(btnType, floor, shouldLightUp)
		}
	}
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
