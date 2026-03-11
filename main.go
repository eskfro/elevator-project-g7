package main

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/movement"
	"elevator-project-g7/internal/network"
	"elevator-project-g7/internal/ordercontrol"
	"elevator-project-g7/internal/parser"
	"elevator-project-g7/internal/rolemanager"
	"log"
	_ "net/http/pprof"
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
	ticker_printElevator := time.NewTicker(1000 * time.Millisecond)
	ticker_AliveList := time.NewTicker(500 * time.Millisecond)
	defer ticker_printElevator.Stop()
	defer ticker_AliveList.Stop()

	// PhysicalInfo updates [critical]
	ch_updateMV_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 20)
	ch_updateOC_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 20)
	ch_updateRM_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 20)
	ch_updateTX_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 20)

	ch_fromIO_Floor := make(chan int, 20)
	ch_fromIO_Obstruction := make(chan bool, 20)
	ch_fromIO_BtnPress := make(chan elevio.ButtonEvent, 50)
	ch_toMV_FloorArrival := make(chan int, 5)

	// From Movement
	ch_fromMV_LOT := make(chan elev.LocalOrderTable, 20)
	ch_fromMV_MotorDir := make(chan elevio.MotorDirection, 20)
	ch_fromMV_Movement := make(chan elev.ElevatorMovement, 20)
	ch_fromMV_ClearOrders := make(chan elev.ClearOrders, 20)

	// To OrderControl
	ch_updateOC_AllOrderTables := make(chan elev.AllOrderTables, 20)
	ch_updateOC_AliveList := make(chan elev.AliveList, 50)

	// From OrderControl
	ch_fromOC_LOT := make(chan elev.LocalOrderTable, 10)
	ch_fromOC_OrderTable := make(chan elev.OrderTable, 10)

	// From RoleManager
	ch_fromRM_Role := make(chan elev.ElevatorRole, 5)
	ch_fromRM_DeadElevId := make(chan int, 5)
	ch_fromRM_PrimaryId := make(chan int, 5)
	ch_fromRM_AliveList := make(chan elev.AliveList, 10)

	// To Network
	ch_updateTX_OTP := make(chan elev.OrderTablePacket, 50)
	updateTX_Role := make(chan elev.ElevatorRole, 5)
	updateRX_Role := make(chan elev.ElevatorRole, 5)
	updateRX_PrimaryId := make(chan int, 5)

	// From Network
	ch_fromRX_OrderTableP := make(chan elev.OrderTablePacket, 50)
	ch_fromRX_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 50)

	go elevio.PollObstructionSwitch(ch_fromIO_Obstruction)
	go elevio.PollFloorSensor(ch_fromIO_Floor)
	go elevio.PollButtons(ch_fromIO_BtnPress)

	go movement.Movement(elevator, ch_updateMV_PhysicalInfo, ch_fromMV_LOT,
		ch_fromMV_Movement, ch_fromMV_MotorDir, ch_fromMV_ClearOrders, ch_toMV_FloorArrival)
	go ordercontrol.OrderControl(elevator, ch_updateOC_AllOrderTables, ch_updateOC_PhysicalInfo, ch_updateOC_AliveList, ch_fromRX_OrderTableP,
		ch_fromOC_LOT, ch_fromOC_OrderTable, ch_updateTX_OTP, ch_fromMV_ClearOrders, ch_fromIO_BtnPress)
	go rolemanager.RoleManager(elevator, ch_updateRM_PhysicalInfo, ch_fromRM_Role, ch_fromRM_DeadElevId, ch_fromRM_PrimaryId, ch_fromRX_PhysicalInfo, ch_fromRM_AliveList)
	go network.TxHeartBeat(elevator, ports.HeartBeat, ch_updateTX_PhysicalInfo)
	go network.RxHeartBeat(ports.HeartBeat, ch_fromRX_PhysicalInfo, elevator.PhysicalInfo.Id)
	go network.TxOrderTableUDP(elevator, ports.OrderTableP, ch_updateTX_OTP, updateTX_Role)
	go network.RxOrderTableUDP(elevator, ports.OrderTableP, ch_fromRX_OrderTableP, updateRX_Role, updateRX_PrimaryId)

	// go func() {
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()

	go func() {
		for {
			select {
			case <-ticker_printElevator.C:
				uptime := time.Since(timeStart).Seconds()
				elev.PrintElevatorInfo(elevator, uptime)
				//elev.PrintOrderTableSlice(elevator.OrderTable, elevator.PhysicalInfo.Id)

				elev.PrintOrderTableSlice(elevator.OrderTable, 0)
				elev.PrintOrderTableSlice(elevator.OrderTable, 1)
				elev.PrintOrderTableSlice(elevator.OrderTable, 2)

			// =========================== FROM HARDWARE ============================

			case obst := <-ch_fromIO_Obstruction:
				log.Println("[MAIN] FromIO obs")
				elevator.PhysicalInfo.Obstructed = obst
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, ch_updateRM_PhysicalInfo, ch_updateOC_PhysicalInfo, ch_updateTX_PhysicalInfo, ch_updateMV_PhysicalInfo)

			case floor := <-ch_fromIO_Floor:
				log.Println("[MAIN] FromIO floor")
				elevator.PhysicalInfo.Floor = floor
				ch_toMV_FloorArrival <- floor
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, ch_updateRM_PhysicalInfo, ch_updateOC_PhysicalInfo, ch_updateTX_PhysicalInfo)

			// =========================== FROM MOVEMENT ============================

			case newLOT := <-ch_fromMV_LOT:
				log.Println("[MAIN] From MV: LocalOrderTable")
				elevator.PhysicalInfo.LocalOrderTable = newLOT
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, ch_updateRM_PhysicalInfo, ch_updateOC_PhysicalInfo, ch_updateTX_PhysicalInfo)

			case newMovement := <-ch_fromMV_Movement:
				log.Println("[MAIN] From MV: Movement")
				elevator.PhysicalInfo.Movement = newMovement
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, ch_updateRM_PhysicalInfo, ch_updateOC_PhysicalInfo, ch_updateTX_PhysicalInfo)

			case newMotorDir := <-ch_fromMV_MotorDir:
				log.Println("[MAIN] From MV: MotorDir")
				elevator.PhysicalInfo.MotorDir = newMotorDir
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, ch_updateRM_PhysicalInfo, ch_updateOC_PhysicalInfo, ch_updateTX_PhysicalInfo)

			// ========================== FROM ORDERCONTROL ============================

			case newOrderTable := <-ch_fromOC_OrderTable:
				log.Println("[MAIN] From OC: OrderTable")
				elevator.OrderTable = newOrderTable

			case newLocalOrderTable := <-ch_fromOC_LOT:
				log.Println("[MAIN] From OC: LocalOrderTable")
				elevator.PhysicalInfo.LocalOrderTable = newLocalOrderTable
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, ch_updateMV_PhysicalInfo, ch_updateRM_PhysicalInfo)

			// ========================== FROM ROLEMANAGER ============================

			case deadElevId := <-ch_fromRM_DeadElevId:
				log.Println("[MAIN] From RM: Dead Elev ID")
				elevator.AliveList[deadElevId].Role = elev.ER_Dead
				ch_updateOC_AliveList <- elevator.AliveList

			case newRole := <-ch_fromRM_Role:
				log.Println("[MAIN] From RM: Role")
				elevator.PhysicalInfo.Role = newRole
				updateTX_Role <- elevator.PhysicalInfo.Role
				updateRX_Role <- elevator.PhysicalInfo.Role
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, ch_updateMV_PhysicalInfo, ch_updateOC_PhysicalInfo, ch_updateTX_PhysicalInfo)

			case newPrimaryId := <-ch_fromRM_PrimaryId:
				log.Println("[MAIN] From RM: Primary ID")
				elevator.PhysicalInfo.PrimaryId = newPrimaryId
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, ch_updateMV_PhysicalInfo, ch_updateOC_PhysicalInfo, ch_updateTX_PhysicalInfo)
				updateRX_PrimaryId <- elevator.PhysicalInfo.PrimaryId

			case newAliveList := <-ch_fromRM_AliveList:
				log.Println("[MAIN] From RM: New AliveList")
				elevator.AliveList = newAliveList
				elevator.NumElevs = rolemanager.CountNumElevs(elevator.AliveList)
				ch_updateOC_AliveList <- elevator.AliveList

				// ========================= FROM NETWORK ============================
			}
		}
	}()

	WaitForInterrupt()
}

func sendPhysicalInfoUpdate(info elev.ElevatorPhysicalInfo, channels ...chan<- elev.ElevatorPhysicalInfo) {
	for _, ch := range channels {
		select {
		case ch <- info:
		default:
			log.Printf("[MAIN] Send Physical Info Default Case")
		}
	}
}
