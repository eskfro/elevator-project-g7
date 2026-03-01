package main

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/movement"
	"elevator-project-g7/internal/network"
	"elevator-project-g7/internal/ordercontrol"
	"elevator-project-g7/internal/parser"
	"elevator-project-g7/internal/rolemanager"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func WaitForInterrupt() {
	ch_sig := make(chan os.Signal, 1)
	signal.Notify(ch_sig, os.Interrupt, syscall.SIGTERM)
	<-ch_sig
}

const sim bool = false

func main() {
	// PARSE ARGS
	id, port_HW, port_HB, port_OT := parser.ParseOsArgs(os.Args, sim)

	// INIT
	elevio.InitPhysicalElevator("localhost", port_HW, elev.N_FLOORS)
	elevator := elev.CreateElevator(id, port_HW, network.GetLocalIP())

	// Time for debugging
	ticker_printElevator := time.NewTicker(2000 * time.Millisecond)
	ticker_AliveList := time.NewTicker(500 * time.Millisecond)
	timeStart := time.Now()
	defer ticker_printElevator.Stop()
	defer ticker_AliveList.Stop()

	// POLLING
	ch_PollObstruction := make(chan bool)
	ch_PollFloorSensor := make(chan int)
	ch_PollButtonPress := make(chan elevio.ButtonEvent)

	// To Movement
	ch_updateMV_Physicalnfo := make(chan elev.ElevatorPhysicalInfo, 5)
	ch_toMV_FloorArrival := make(chan int, 5)

	// From Movement
	ch_fromMV_LOT := make(chan elev.LocalOrderTable, 2)
	ch_fromMV_State := make(chan elev.ElevatorMovement, 2)
	ch_fromMV_MotorDir := make(chan elevio.MotorDirection, 2)

	// To OrderControl
	ch_toOC_OrderTableP := make(chan elev.OrderTablePacket, 4)
	ch_updateOC_OrderTable := make(chan elev.OrderTable, 4)
	ch_updateOC_AllOrderTables := make(chan elev.AllOrderTables, 4)
	ch_updateOC_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 4)
	ch_updateOC_AliveList := make(chan elev.AliveList, 4)
	ch_updateOC_NumElevs := make(chan int, 4)

	// From OrderControl
	ch_fromOC_ClearOrder := make(chan elevio.ButtonEvent, 4)
	ch_fromOC_LOT := make(chan elev.LocalOrderTable, 4)
	ch_fromOC_OrderTable := make(chan elev.OrderTable, 4)

	// To RoleManager
	ch_toRM_HeartBeatId := make(chan int, 4)
	ch_updateRM_AliveList := make(chan elev.AliveList, 4)
	ch_updateRM_NumElevs := make(chan int, 4)
	ch_updateRM_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 4)

	// From RoleManager
	ch_fromRM_Role := make(chan elev.ElevatorRole, 4)
	ch_fromRM_DeadElevId := make(chan int, 4)
	ch_fromRM_NumElevs := make(chan int, 4)

	// To Network
	ch_TxOrderTableP := make(chan elev.OrderTablePacket, 4)
	ch_UpdateTxMessage := make(chan elev.ElevatorPhysicalInfo, 4)

	// From Network
	ch_RxOrderTableP := make(chan elev.OrderTablePacket, 4)
	ch_RxPhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 4) //made this buffered

	// ============ GO SENSOR POLLING ==================

	go elevio.PollObstructionSwitch(ch_PollObstruction)
	go elevio.PollFloorSensor(ch_PollFloorSensor)
	go elevio.PollButtons(ch_PollButtonPress)

	// ============ GO MOVEMENT ========================

	go movement.Movement(elevator, ch_updateMV_Physicalnfo, ch_toMV_FloorArrival, ch_fromMV_LOT, ch_fromMV_State, ch_fromMV_MotorDir)

	// ============= GO ORDERCONTROL ===================

	go ordercontrol.OrderControl(elevator, ch_updateOC_OrderTable, ch_updateOC_AllOrderTables, ch_updateOC_PhysicalInfo, ch_updateOC_AliveList, ch_updateOC_NumElevs, ch_toOC_OrderTableP, ch_fromOC_LOT, ch_fromOC_OrderTable)

	// ============= GO NETWORK ========================

	go network.Transmitter(port_OT, ch_TxOrderTableP) // TODO: change function name
	go network.Receiver(port_OT, ch_RxOrderTableP)    // TODO: change function name
	go network.TxHeartBeat(elevator, port_HB, ch_UpdateTxMessage)
	go network.RxHeartBeat(port_HB, ch_RxPhysicalInfo, elevator.PhysicalInfo.Id)

	// ============= GO ROLE MANAGER ===================

	go rolemanager.RoleManager(elevator, ch_updateRM_AliveList, ch_updateRM_PhysicalInfo, ch_updateRM_NumElevs, ch_toRM_HeartBeatId, ch_fromRM_Role, ch_fromRM_DeadElevId, ch_fromRM_NumElevs)

	go func() {

		for {
			select {

			case <-ticker_printElevator.C:
				uptime := time.Since(timeStart).Seconds()
				elev.PrintElevatorInfo(elevator, uptime)

			// ================================ MOVEMENT ============================
			// TO
			case obst := <-ch_PollObstruction:

				elevator.PhysicalInfo.Obstructed = obst
				ch_UpdateTxMessage <- elevator.PhysicalInfo
				ch_updateMV_Physicalnfo <- elevator.PhysicalInfo

			case floor := <-ch_PollFloorSensor:

				elevator.PhysicalInfo.Floor = floor
				ch_toMV_FloorArrival <- floor
				ch_UpdateTxMessage <- elevator.PhysicalInfo
				ch_updateOC_PhysicalInfo <- elevator.PhysicalInfo

			// FROM
			case newLOT := <-ch_fromMV_LOT:

				elevator.PhysicalInfo.LocalOrderTable = newLOT
				ch_UpdateTxMessage <- elevator.PhysicalInfo

			case newState := <-ch_fromMV_State:

				elevator.PhysicalInfo.State = newState
				ch_UpdateTxMessage <- elevator.PhysicalInfo
				ch_updateOC_PhysicalInfo <- elevator.PhysicalInfo

			case newMotorDir := <-ch_fromMV_MotorDir:

				elevator.PhysicalInfo.MotorDir = newMotorDir
				ch_UpdateTxMessage <- elevator.PhysicalInfo
				ch_updateOC_PhysicalInfo <- elevator.PhysicalInfo

			// ================================ ORDERCONTROL ============================

			// TO
			case btnPress := <-ch_PollButtonPress:

				elevator.OrderTable[elevator.PhysicalInfo.Id][btnPress.Floor][btnPress.Button] = elev.OS_CONFIRMED //For testing I put this OS_CONFIRMED
				elevio.PrintButtonpress(btnPress)
				ch_updateOC_OrderTable <- elevator.OrderTable

			// FROM
			case order := <-ch_fromOC_ClearOrder:

				elevator.OrderTable[elevator.PhysicalInfo.Id][order.Floor][order.Button] = elev.OS_CLEAR

			case newOrderTable := <-ch_fromOC_OrderTable:

				elevator.OrderTable = newOrderTable

			case newLocalOrderTable := <-ch_fromOC_LOT:

				elevator.PhysicalInfo.LocalOrderTable = newLocalOrderTable
				ch_updateMV_Physicalnfo <- elevator.PhysicalInfo

			// ================================ ROLEMANAGER ============================

			// TO

			// FROM

			case <-ticker_AliveList.C:

				ch_updateRM_AliveList <- elevator.AliveList

			case deadElevId := <-ch_fromRM_DeadElevId:

				elevator.AliveList[deadElevId].Role = elev.ER_Dead
				ch_updateRM_AliveList <- elevator.AliveList

			case newRole := <-ch_fromRM_Role:

				elevator.PhysicalInfo.Role = newRole
				if newRole == elev.ER_Primary {
					elevator.PhysicalInfo.PrimaryId = elevator.PhysicalInfo.Id
				}
				ch_UpdateTxMessage <- elevator.PhysicalInfo

			case newNumElevs := <-ch_fromRM_NumElevs:

				elevator.NumElevs = newNumElevs

			// ================================ NETWORK ============================

			// TO

			// FROM

			// Denne her er både From Network og To OrderControl
			case packet := <-ch_RxOrderTableP:

				if elevator.PhysicalInfo.Role == elev.ER_Primary {

					elevator.AllOrderTables[packet.Id] = packet.OrderTable
					ch_updateOC_AllOrderTables <- elevator.AllOrderTables

				}
				ch_toOC_OrderTableP <- packet

			case heartBeat := <-ch_RxPhysicalInfo:

				elevator.AliveList[heartBeat.Id] = heartBeat
				ch_updateRM_AliveList <- elevator.AliveList
				// Send id for starting heartbeat timer
				ch_toRM_HeartBeatId <- heartBeat.Id

			}

		}
	}()

	WaitForInterrupt()

}
