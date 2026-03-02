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
	id, ports := parser.ParseOsArgs(os.Args, sim)

	// INIT
	elevio.InitPhysicalElevator("localhost", ports.Hardware, elev.N_FLOORS)
	elevator := elev.CreateElevator(id, ports.Hardware, network.GetLocalIP())
	elev.PrintElevatorInit(id, ports.Hardware)
	elev.PrintElevatorInfo(elevator, 0)

	// Time for debugging
	timeStart := time.Now()
	ticker_printElevator := time.NewTicker(2000 * time.Millisecond)
	ticker_AliveList := time.NewTicker(500 * time.Millisecond)
	defer ticker_printElevator.Stop()
	defer ticker_AliveList.Stop()

	// POLLING
	ch_PollObstruction := make(chan bool, 10)
	ch_PollFloorSensor := make(chan int, 10)
	ch_PollButtonPress := make(chan elevio.ButtonEvent, 10)

	// To Movement
	ch_updateMV_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 5)
	ch_toMV_FloorArrival := make(chan int, 5)

	// From Movement
	ch_fromMV_LOT := make(chan elev.LocalOrderTable, 2)
	ch_fromMV_Movement := make(chan elev.ElevatorMovement, 2)
	ch_fromMV_MotorDir := make(chan elevio.MotorDirection, 2)

	// To OrderControl
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
	ch_fromRM_PrimaryId := make(chan int, 4)

	// To Network
	ch_TxOrderTableP := make(chan elev.OrderTablePacket, 4)
	ch_UpdateTxMessage := make(chan elev.ElevatorPhysicalInfo, 20)

	// From Network
	ch_RxOrderTableP := make(chan elev.OrderTablePacket, 4)
	ch_RxPhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 20)

	go elevio.PollObstructionSwitch(ch_PollObstruction)
	go elevio.PollFloorSensor(ch_PollFloorSensor)
	go elevio.PollButtons(ch_PollButtonPress)
	go movement.Movement(elevator, ch_updateMV_PhysicalInfo, ch_toMV_FloorArrival, ch_fromMV_LOT, ch_fromMV_Movement, ch_fromMV_MotorDir)
	go ordercontrol.OrderControl(elevator, ch_updateOC_OrderTable, ch_updateOC_AllOrderTables, ch_updateOC_PhysicalInfo, ch_updateOC_AliveList, ch_updateOC_NumElevs, ch_RxOrderTableP, ch_fromOC_LOT, ch_fromOC_OrderTable)
	go network.Transmitter(ports.OrderTableP, ch_TxOrderTableP) // TODO: change function name
	go network.Receiver(ports.OrderTableP, ch_RxOrderTableP)    // TODO: change function name
	go network.TxHeartBeat(elevator, ports.HeartBeat, ch_UpdateTxMessage)
	go network.RxHeartBeat(ports.HeartBeat, ch_RxPhysicalInfo, elevator.PhysicalInfo.Id)
	go rolemanager.RoleManager(elevator, ch_updateRM_AliveList, ch_updateRM_PhysicalInfo, ch_updateRM_NumElevs, ch_toRM_HeartBeatId, ch_fromRM_Role, ch_fromRM_DeadElevId, ch_fromRM_NumElevs, ch_fromRM_PrimaryId)

	go func() {

		for {
			select {

			case <-ticker_printElevator.C:
				uptime := time.Since(timeStart).Seconds()
				elev.PrintElevatorInfo(elevator, uptime)

			// ========================== FROM HARDWARE =========================

			case btnPress := <-ch_PollButtonPress:

				elevio.PrintButtonpress(btnPress)
				elevator.OrderTable[elevator.PhysicalInfo.Id][btnPress.Floor][btnPress.Button] = elev.OS_REQUESTED
				packet := elev.OrderTablePacket{Id: elevator.PhysicalInfo.Id, OrderTable: elevator.OrderTable}
				ch_TxOrderTableP <- packet

				/*
					CODE FOR SINGLE ELEVATOR
					ch_updateOC_OrderTable <- elevator.OrderTable
					-> Also, it was hardcoded to elev.OS_CONFIRMED
				*/

			case obst := <-ch_PollObstruction:

				elevator.PhysicalInfo.Obstructed = obst
				ch_UpdateTxMessage <- elevator.PhysicalInfo
				ch_updateMV_PhysicalInfo <- elevator.PhysicalInfo

			case floor := <-ch_PollFloorSensor:

				elevator.PhysicalInfo.Floor = floor
				ch_toMV_FloorArrival <- floor
				ch_UpdateTxMessage <- elevator.PhysicalInfo
				ch_updateOC_PhysicalInfo <- elevator.PhysicalInfo

			// =========================== FROM MOVEMENT ============================

			case newLOT := <-ch_fromMV_LOT:

				elevator.PhysicalInfo.LocalOrderTable = newLOT
				f := elevator.PhysicalInfo.Floor
				for b := 0; b < elev.N_BUTTONS; b++ {
					if !newLOT[f][b] {
						elevator.OrderTable[elevator.PhysicalInfo.Id][f][b] = elev.OS_NO_ORDER
					}
				}
				ch_UpdateTxMessage <- elevator.PhysicalInfo

			case newState := <-ch_fromMV_Movement:

				elevator.PhysicalInfo.Movement = newState
				ch_UpdateTxMessage <- elevator.PhysicalInfo
				ch_updateOC_PhysicalInfo <- elevator.PhysicalInfo

			case newMotorDir := <-ch_fromMV_MotorDir:

				elevator.PhysicalInfo.MotorDir = newMotorDir
				ch_UpdateTxMessage <- elevator.PhysicalInfo
				ch_updateOC_PhysicalInfo <- elevator.PhysicalInfo

			// ========================== FROM ORDERCONTROL ============================

			case order := <-ch_fromOC_ClearOrder:

				elevator.OrderTable[elevator.PhysicalInfo.Id][order.Floor][order.Button] = elev.OS_CLEAR

			case newOrderTable := <-ch_fromOC_OrderTable:

				elevator.OrderTable = newOrderTable

			case newLocalOrderTable := <-ch_fromOC_LOT:

				elevator.PhysicalInfo.LocalOrderTable = newLocalOrderTable
				ch_updateMV_PhysicalInfo <- elevator.PhysicalInfo
				ch_updateRM_PhysicalInfo <- elevator.PhysicalInfo

			// ========================== FROM ROLEMANAGER ============================

			case <-ticker_AliveList.C:

				ch_updateRM_AliveList <- elevator.AliveList

			case deadElevId := <-ch_fromRM_DeadElevId:

				elevator.AliveList[deadElevId].Role = elev.ER_Dead
				ch_updateRM_AliveList <- elevator.AliveList

			case newRole := <-ch_fromRM_Role:

				elevator.PhysicalInfo.Role = newRole
				ch_UpdateTxMessage <- elevator.PhysicalInfo
				ch_updateMV_PhysicalInfo <- elevator.PhysicalInfo
				ch_updateOC_PhysicalInfo <- elevator.PhysicalInfo

			case newNumElevs := <-ch_fromRM_NumElevs:

				elevator.NumElevs = newNumElevs

			case newPrimaryId := <-ch_fromRM_PrimaryId:

				elevator.PhysicalInfo.PrimaryId = newPrimaryId
				ch_updateMV_PhysicalInfo <- elevator.PhysicalInfo
				ch_updateOC_PhysicalInfo <- elevator.PhysicalInfo

			// ========================= FROM NETWORK ============================

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
