package eventloop

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/movement"
	"elevator-project-g7/internal/network"
	"elevator-project-g7/internal/ordercontrol"
	"elevator-project-g7/internal/rolemanager"
	"log"
	"time"
)

func sendPhysicalInfoUpdate(info elev.ElevatorPhysicalInfo, channels ...chan<- elev.ElevatorPhysicalInfo) {
	for _, ch := range channels {
		select {
		case ch <- info:
		default:
			log.Printf("[MAIN] Send Physical Info Default Case")
		}
	}
}

func Start(
	elevator elev.Elevator,
	ports network.Ports,
	fromIO_BtnPress <-chan elevio.ButtonEvent,
	fromIO_Floor <-chan int,
	fromIO_Obstruction <-chan bool,
) {
	// Ticker for debugging
	timeStart := time.Now()
	ticker_printElevator := time.NewTicker(1000 * time.Millisecond)
	defer ticker_printElevator.Stop()

	// =================================================================== CHANNELS
	// Movement
	updateMV_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 20)
	toMV_FloorArrival := make(chan int, 5)
	fromMV_LOT := make(chan elev.LocalOrderTable, 20)
	fromMV_MotorDir := make(chan elevio.MotorDirection, 20)
	fromMV_Movement := make(chan elev.ElevatorMovement, 20)
	fromMV_ClearOrders := make(chan elev.ClearOrders, 20)

	// OrderControl
	updateOC_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 20)
	updateOC_AliveList := make(chan elev.AliveList, 50)
	fromOC_OrderTable := make(chan elev.OrderTable, 20)

	// RoleManager
	updateRM_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 20)
	fromRM_Role := make(chan elev.ElevatorRole, 20)
	fromRM_PrimaryId := make(chan int, 20)
	fromRM_AliveList := make(chan elev.AliveList, 20)
	fromRM_ResetVersion := make(chan int, 10)

	// Transmitter
	updateTX_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 20)
	updateTX_OTP := make(chan elev.OrderTablePacket, 100)
	updateTX_Role := make(chan elev.ElevatorRole, 5)

	// Reciever
	updateRX_Role := make(chan elev.ElevatorRole, 5)
	updateRX_PrimaryId := make(chan int, 5)
	fromRX_OrderTableP := make(chan elev.OrderTablePacket, 100)
	fromRX_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 100)

	// Start modules
	go movement.Movement(elevator, updateMV_PhysicalInfo, fromMV_LOT, fromMV_Movement, fromMV_MotorDir, fromMV_ClearOrders, toMV_FloorArrival)
	go ordercontrol.OrderControl(elevator, updateOC_PhysicalInfo, updateOC_AliveList, fromRX_OrderTableP, fromMV_ClearOrders, fromIO_BtnPress, fromOC_OrderTable, updateTX_OTP)
	go rolemanager.RoleManager(elevator, fromRX_PhysicalInfo, updateRM_PhysicalInfo, fromRM_Role, fromRM_PrimaryId, fromRM_AliveList, fromRM_ResetVersion)
	go network.TxHeartBeat(elevator, ports.HeartBeat, updateTX_PhysicalInfo)
	go network.RxHeartBeat(ports.HeartBeat, fromRX_PhysicalInfo, elevator.PhysicalInfo.Id)
	go network.TxOrderTable(elevator, ports.OrderTableP, updateTX_OTP, updateTX_Role)
	go network.RxOrderTable(elevator, ports.OrderTableP, updateRX_Role, updateRX_PrimaryId, fromRM_ResetVersion, fromRX_OrderTableP)

	// ========================================================================= PRINT DEBUGGING
	go func() {
		for range ticker_printElevator.C {
			uptime := time.Since(timeStart).Seconds()
			elev.PrintElevatorInfo(elevator, uptime)
			elev.PrintOrderTableSlice(elevator.OrderTable, elevator.PhysicalInfo.Id)
		}
	}()

	func() {
		for {
			select {
			// ================================================================= FROM HARDWARE

			case obst := <-fromIO_Obstruction:
				log.Println("[MAIN] FromIO obs")
				elevator.PhysicalInfo.Obstructed = obst
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateRM_PhysicalInfo, updateOC_PhysicalInfo, updateTX_PhysicalInfo, updateMV_PhysicalInfo)

			case floor := <-fromIO_Floor:
				log.Println("[MAIN] FromIO floor")
				elevator.PhysicalInfo.Floor = floor
				toMV_FloorArrival <- floor
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateRM_PhysicalInfo, updateOC_PhysicalInfo, updateTX_PhysicalInfo)

			// ================================================================== FROM MOVEMENT

			case newLOT := <-fromMV_LOT:
				log.Println("[MAIN] From MV: LocalOrderTable")
				elevator.PhysicalInfo.LocalOrderTable = newLOT
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateRM_PhysicalInfo, updateOC_PhysicalInfo, updateTX_PhysicalInfo)

			case newMovement := <-fromMV_Movement:
				log.Println("[MAIN] From MV: Movement")
				elevator.PhysicalInfo.Movement = newMovement
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateRM_PhysicalInfo, updateOC_PhysicalInfo, updateTX_PhysicalInfo)

			case newMotorDir := <-fromMV_MotorDir:
				log.Println("[MAIN] From MV: MotorDir")
				elevator.PhysicalInfo.MotorDir = newMotorDir
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateRM_PhysicalInfo, updateOC_PhysicalInfo, updateTX_PhysicalInfo)

			// ================================================================== FROM ORDERCONTROL

			case newOrderTable := <-fromOC_OrderTable:
				log.Println("[MAIN] From OC: OrderTable")
				isLOTChanged := elevator.PhysicalInfo.LocalOrderTable != ordercontrol.OrderTableToLOT(newOrderTable, elevator.PhysicalInfo.Id)
				isOTChanged := elevator.OrderTable != newOrderTable

				elevator.OrderTable = newOrderTable
				elevator.PhysicalInfo.LocalOrderTable = ordercontrol.OrderTableToLOT(elevator.OrderTable, elevator.PhysicalInfo.Id)

				if isLOTChanged {
					sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateMV_PhysicalInfo)
				}
				if isOTChanged {
					elev.SetAllLights(elevator.PhysicalInfo.LocalOrderTable, elevator.OrderTable, elevator.AliveList)
				}

			// =================================================================== FROM ROLEMANAGER

			case newRole := <-fromRM_Role:
				log.Println("[MAIN] From RM: Role")
				isChanged := elevator.PhysicalInfo.Role != newRole
				elevator.PhysicalInfo.Role = newRole

				if isChanged {
					updateTX_Role <- elevator.PhysicalInfo.Role
					updateRX_Role <- elevator.PhysicalInfo.Role
					sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateMV_PhysicalInfo, updateOC_PhysicalInfo, updateTX_PhysicalInfo)
				}

			case newPrimaryId := <-fromRM_PrimaryId:
				log.Println("[MAIN] From RM: Primary ID")
				isChanged := elevator.PhysicalInfo.PrimaryId != newPrimaryId
				elevator.PhysicalInfo.PrimaryId = newPrimaryId

				if isChanged {
					sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateMV_PhysicalInfo, updateOC_PhysicalInfo, updateTX_PhysicalInfo)
					updateRX_PrimaryId <- elevator.PhysicalInfo.PrimaryId
				}

			case newAliveList := <-fromRM_AliveList:
				log.Println("[MAIN] From RM: New AliveList")
				isChanged := elevator.AliveList != newAliveList
				elevator.AliveList = newAliveList
				elevator.NumElevs = rolemanager.CountNumElevs(elevator.AliveList)

				if isChanged {
					updateOC_AliveList <- elevator.AliveList
					elev.SetAllLights(elevator.PhysicalInfo.LocalOrderTable, elevator.OrderTable, elevator.AliveList)
				}
			}
		}
	}()
}
