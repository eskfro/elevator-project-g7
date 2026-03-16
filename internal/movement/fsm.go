package movement

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/requests"
	"elevator-project-g7/internal/timer"
	"fmt"
)

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
