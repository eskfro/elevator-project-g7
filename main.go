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
	ticker_printElevator := time.NewTicker(500 * time.Millisecond)
	ticker_AliveList := time.NewTicker(500 * time.Millisecond)
	defer ticker_printElevator.Stop()
	defer ticker_AliveList.Stop()

	// POLLING
	ch_PollObstruction := make(chan bool, 10)
	ch_PollFloorSensor := make(chan int, 10)
	ch_PollButtonPress := make(chan elevio.ButtonEvent, 10)

	// PhysicalInfo updates [critical]
	ch_updateMV_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 5)
	ch_updateOC_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 4)
	ch_updateRM_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 4)
	ch_updateTX_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 4)

	// From Movement
	ch_toMV_FloorArrival := make(chan int, 5)
	ch_fromMV_LOT := make(chan elev.LocalOrderTable, 2)
	ch_fromMV_MotorDir := make(chan elevio.MotorDirection, 2)
	ch_fromMV_Movement := make(chan elev.ElevatorMovement, 2)
	ch_fromMV_ClearOrder := make(chan elev.Order, 4)

	// To OrderControl
	ch_updateOC_AllOrderTables := make(chan elev.AllOrderTables, 4)
	ch_updateOC_NumElevs := make(chan int, 4)
	ch_updateOC_AliveList := make(chan elev.AliveList, 4)

	// From OrderControl
	ch_fromOC_LOT := make(chan elev.LocalOrderTable, 4)
	ch_fromOC_OrderTable := make(chan elev.OrderTable, 4)

	// To RoleManager
	ch_toRM_HeartBeatId := make(chan int, 4)
	ch_updateRM_NumElevs := make(chan int, 4)
	ch_updateRM_AliveList := make(chan elev.AliveList, 4)

	// From RoleManager
	ch_fromRM_Role := make(chan elev.ElevatorRole, 4)
	ch_fromRM_DeadElevId := make(chan int, 4)
	ch_fromRM_NumElevs := make(chan int, 4)
	ch_fromRM_PrimaryId := make(chan int, 4)

	// To Network
	ch_updateTX_OTP := make(chan elev.OrderTablePacket, 4)
	ch_updateTX_PacketId := make(chan int, 4)

	// From Network
	ch_fromRX_OrderTableP := make(chan elev.OrderTablePacket, 4)
	ch_fromRX_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 20)

	go elevio.PollObstructionSwitch(ch_PollObstruction)
	go elevio.PollFloorSensor(ch_PollFloorSensor)
	go elevio.PollButtons(ch_PollButtonPress)
	go movement.Movement(elevator, ch_updateMV_PhysicalInfo, ch_toMV_FloorArrival, ch_fromMV_LOT, ch_fromMV_Movement, ch_fromMV_MotorDir, ch_fromMV_ClearOrder)
	go ordercontrol.OrderControl(elevator, ch_updateOC_AllOrderTables, ch_updateOC_PhysicalInfo, ch_updateOC_AliveList, ch_updateOC_NumElevs, ch_fromRX_OrderTableP, ch_fromOC_LOT, ch_fromOC_OrderTable)
	go network.TxHeartBeat(elevator, ports.HeartBeat, ch_updateTX_OTP, ch_updateTX_PacketId, ch_updateTX_PhysicalInfo)
	go network.RxHeartBeat(ports.HeartBeat, ch_fromRX_OrderTableP, ch_fromRX_PhysicalInfo, elevator.PhysicalInfo.Id)
	go rolemanager.RoleManager(elevator, ch_updateRM_AliveList, ch_updateRM_PhysicalInfo, ch_updateRM_NumElevs, ch_toRM_HeartBeatId, ch_fromRM_Role, ch_fromRM_DeadElevId, ch_fromRM_NumElevs, ch_fromRM_PrimaryId)

	go func() {
		for {
			select {
			case <-ticker_printElevator.C:
				uptime := time.Since(timeStart).Seconds()
				elev.PrintElevatorInfo(elevator, uptime)
				elev.PrintOrderTableSlice(elevator.OrderTable, elevator.PhysicalInfo.Id)

			// ========================== POLL SENSORS =========================

			case btnPress := <-ch_PollButtonPress:
				elevio.PrintButtonpress(btnPress)
				elevator.OrderTable[elevator.PhysicalInfo.Id][btnPress.Floor][btnPress.Button] = elev.OS_REQUESTED
				ch_updateTX_OTP <- elev.OrderTablePacket{Id: elevator.PhysicalInfo.Id, OrderTable: elevator.OrderTable}

			case obst := <-ch_PollObstruction:
				elevator.PhysicalInfo.Obstructed = obst
				ch_updateTX_PhysicalInfo <- elevator.PhysicalInfo
				ch_updateMV_PhysicalInfo <- elevator.PhysicalInfo
				ch_updateRM_PhysicalInfo <- elevator.PhysicalInfo

			case floor := <-ch_PollFloorSensor:
				elevator.PhysicalInfo.Floor = floor
				ch_toMV_FloorArrival <- floor
				ch_updateTX_PhysicalInfo <- elevator.PhysicalInfo
				ch_updateOC_PhysicalInfo <- elevator.PhysicalInfo
				ch_updateRM_PhysicalInfo <- elevator.PhysicalInfo

			// =========================== FROM MOVEMENT ============================

			case clearOrder := <-ch_fromMV_ClearOrder:
				elevator.OrderTable[clearOrder.ElevatorNumber][clearOrder.Floor][clearOrder.ButtonType] = elev.OS_CLEAR
				ch_updateTX_OTP <- elev.OrderTablePacket{Id: elevator.PhysicalInfo.Id, OrderTable: elevator.OrderTable}

			case newLOT := <-ch_fromMV_LOT:
				elevator.PhysicalInfo.LocalOrderTable = newLOT
				ch_updateTX_PhysicalInfo <- elevator.PhysicalInfo
				ch_updateOC_PhysicalInfo <- elevator.PhysicalInfo
				ch_updateRM_PhysicalInfo <- elevator.PhysicalInfo

			case newState := <-ch_fromMV_Movement:
				elevator.PhysicalInfo.Movement = newState
				ch_updateTX_PhysicalInfo <- elevator.PhysicalInfo
				ch_updateOC_PhysicalInfo <- elevator.PhysicalInfo
				ch_updateRM_PhysicalInfo <- elevator.PhysicalInfo

			case newMotorDir := <-ch_fromMV_MotorDir:
				elevator.PhysicalInfo.MotorDir = newMotorDir
				ch_updateTX_PhysicalInfo <- elevator.PhysicalInfo
				ch_updateOC_PhysicalInfo <- elevator.PhysicalInfo
				ch_updateRM_PhysicalInfo <- elevator.PhysicalInfo

			// ========================== FROM ORDERCONTROL ============================

			case newOrderTable := <-ch_fromOC_OrderTable:
				elevator.OrderTable = newOrderTable
				ch_updateTX_OTP <- elev.OrderTablePacket{Id: elevator.PhysicalInfo.Id, OrderTable: elevator.OrderTable}

			case newLocalOrderTable := <-ch_fromOC_LOT:
				elevator.PhysicalInfo.LocalOrderTable = newLocalOrderTable
				ch_updateMV_PhysicalInfo <- elevator.PhysicalInfo
				ch_updateRM_PhysicalInfo <- elevator.PhysicalInfo

			// ========================== FROM ROLEMANAGER ============================

			// TODO: remove this case maybe.
			case <-ticker_AliveList.C:
				ch_updateRM_AliveList <- elevator.AliveList

			case deadElevId := <-ch_fromRM_DeadElevId:
				elevator.AliveList[deadElevId].Role = elev.ER_Dead
				ch_updateRM_AliveList <- elevator.AliveList
				ch_updateOC_AliveList <- elevator.AliveList

			case newRole := <-ch_fromRM_Role:
				elevator.PhysicalInfo.Role = newRole
				ch_updateTX_PhysicalInfo <- elevator.PhysicalInfo
				ch_updateMV_PhysicalInfo <- elevator.PhysicalInfo
				ch_updateOC_PhysicalInfo <- elevator.PhysicalInfo

			case newNumElevs := <-ch_fromRM_NumElevs:
				elevator.NumElevs = newNumElevs
				ch_updateOC_NumElevs <- newNumElevs

			case newPrimaryId := <-ch_fromRM_PrimaryId:
				elevator.PhysicalInfo.PrimaryId = newPrimaryId
				ch_updateMV_PhysicalInfo <- elevator.PhysicalInfo
				ch_updateOC_PhysicalInfo <- elevator.PhysicalInfo
				ch_updateTX_PhysicalInfo <- elevator.PhysicalInfo

			// ========================= FROM NETWORK ============================

			case heartBeat := <-ch_fromRX_PhysicalInfo:
				elevator.AliveList[heartBeat.Id] = heartBeat
				ch_updateRM_AliveList <- elevator.AliveList
				ch_updateOC_AliveList <- elevator.AliveList
				ch_toRM_HeartBeatId <- heartBeat.Id

			}
		}
	}()

	WaitForInterrupt()

}
