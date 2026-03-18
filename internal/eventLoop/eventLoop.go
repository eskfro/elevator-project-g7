package eventloop

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/hardware"
	"elevator-project-g7/internal/movement"
	"elevator-project-g7/internal/network"
	ordercontrol "elevator-project-g7/internal/orderControl"
	rolemanager "elevator-project-g7/internal/roleManager"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func Start(
	elevator elev.Elevator,
	ports network.Ports,
	fromIO_BtnPress <-chan elevio.ButtonEvent,
	fromIO_Floor <-chan int,
	fromIO_Obstruction <-chan bool,
	processPairsTx chan<- elev.Elevator,
) {
	timeStart := time.Now()
	ticker_printElevator := time.NewTicker(1000 * time.Millisecond)
	defer ticker_printElevator.Stop()

	// =================================================================== CHANNELS
	// Movement
	updateMV_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 32)
	toMV_FloorArrival := make(chan int, 16)
	fromMV_LOT := make(chan elev.LocalOrderTable, 32)
	fromMV_MotorDir := make(chan elevio.MotorDirection, 32)
	fromMV_Movement := make(chan elev.ElevatorMovement, 32)
	fromMV_ClearOrders := make(chan elev.ClearOrders, 32)

	// OrderControl
	updateOC_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 32)
	updateOC_AliveList := make(chan elev.AliveList, 64)
	fromOC_OrderTable := make(chan elev.OrderTable, 256)

	// RoleManager
	updateRM_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 32)
	fromRM_Role := make(chan elev.ElevatorRole, 32)
	fromRM_PrimaryID := make(chan int, 32)
	fromRM_AliveList := make(chan elev.AliveList, 32)
	fromRM_ResetVersion := make(chan int, 32)
	fromRM_ResetNewElevTimer := make(chan struct{}, 32)

	// Network transmitter
	updateTX_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 32)
	updateTX_OTP := make(chan elev.OrderTablePacket, 128)

	// Network reciever
	updateRX_Role := make(chan elev.ElevatorRole, 16)
	updateRX_PrimaryID := make(chan int, 16)
	fromRX_OrderTableP := make(chan elev.OrderTablePacket, 128)
	fromRX_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 128)

	// Hardware
	fromHW_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 128)
	updateHW_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 32)

	// Start modules
	go movement.Movement(elevator, updateMV_PhysicalInfo, fromMV_LOT, fromMV_Movement, fromMV_MotorDir, fromMV_ClearOrders, toMV_FloorArrival)
	go ordercontrol.OrderControl(elevator, updateOC_PhysicalInfo, updateOC_AliveList, fromRX_OrderTableP, fromMV_ClearOrders, fromIO_BtnPress, fromOC_OrderTable, updateTX_OTP, fromRM_ResetNewElevTimer)
	go rolemanager.RoleManager(elevator, fromRX_PhysicalInfo, updateRM_PhysicalInfo, fromRM_Role, fromRM_PrimaryID, fromRM_AliveList, fromRM_ResetVersion, fromRM_ResetNewElevTimer)
	go network.TxHeartBeat(elevator, ports.HeartBeat, updateTX_PhysicalInfo)
	go network.RxHeartBeat(ports.HeartBeat, fromRX_PhysicalInfo, elevator.PhysicalInfo.ID)
	go network.TxOrderTable(elevator, ports.OrderTableP, updateTX_OTP)
	go network.RxOrderTable(elevator, ports.OrderTableP, updateRX_Role, updateRX_PrimaryID, fromRM_ResetVersion, fromRX_OrderTableP)
	go hardware.Start(elevator, fromIO_Obstruction, fromIO_Floor, fromHW_PhysicalInfo, toMV_FloorArrival, updateHW_PhysicalInfo)

	processPairSnapshot(elevator, processPairsTx)

	func() {
		for {
			select {

			case <-ticker_printElevator.C:
				uptime := time.Since(timeStart).Seconds()
				elev.PrintElevatorInfo(elevator, uptime)
				elev.PrintOrderTableSlice(elevator.OrderTable, elevator.PhysicalInfo.ID)

			// ================================================================== FROM HARDWARE
			case newPhysicalInfo := <-fromHW_PhysicalInfo:
				log.Println("[eventLoop] From HW PhysicalInfo ")
				elevator.PhysicalInfo = newPhysicalInfo
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateRM_PhysicalInfo, updateOC_PhysicalInfo, updateTX_PhysicalInfo, updateMV_PhysicalInfo)
				processPairSnapshot(elevator, processPairsTx)

			// ================================================================== FROM MOVEMENT
			case newLOT := <-fromMV_LOT:
				log.Println("[MAIN] From MV: LocalOrderTable")
				elevator.PhysicalInfo.LocalOrderTable = newLOT
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateRM_PhysicalInfo, updateOC_PhysicalInfo, updateTX_PhysicalInfo, updateHW_PhysicalInfo)
				processPairSnapshot(elevator, processPairsTx)

			case newMovement := <-fromMV_Movement:
				log.Println("[MAIN] From MV: Movement")
				elevator.PhysicalInfo.Movement = newMovement
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateRM_PhysicalInfo, updateOC_PhysicalInfo, updateTX_PhysicalInfo, updateHW_PhysicalInfo)
				processPairSnapshot(elevator, processPairsTx)

			case newMotorDir := <-fromMV_MotorDir:
				log.Println("[MAIN] From MV: MotorDir")
				elevator.PhysicalInfo.MotorDir = newMotorDir
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateRM_PhysicalInfo, updateOC_PhysicalInfo, updateTX_PhysicalInfo, updateHW_PhysicalInfo)
				processPairSnapshot(elevator, processPairsTx)

			// ================================================================== FROM ORDERCONTROL

			case newOrderTable := <-fromOC_OrderTable:
				log.Println("[MAIN] From OC: OrderTable")

				elevator.OrderTable = newOrderTable
				elevator.PhysicalInfo.LocalOrderTable = ordercontrol.OrderTableToLOT(elevator.OrderTable, elevator.PhysicalInfo.ID)

				sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateMV_PhysicalInfo, updateHW_PhysicalInfo)
				processPairSnapshot(elevator, processPairsTx)

				elev.SetAllLights(elevator.PhysicalInfo.LocalOrderTable, elevator.OrderTable, elevator.AliveList)

			// =================================================================== FROM ROLEMANAGER

			case newRole := <-fromRM_Role:
				log.Println("[MAIN] From RM: Role")
				isChanged := elevator.PhysicalInfo.Role != newRole
				elevator.PhysicalInfo.Role = newRole

				if isChanged {
					updateRX_Role <- elevator.PhysicalInfo.Role
					sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateMV_PhysicalInfo, updateOC_PhysicalInfo, updateTX_PhysicalInfo, updateHW_PhysicalInfo)
					processPairSnapshot(elevator, processPairsTx)
				}

			case newPrimaryId := <-fromRM_PrimaryID:
				log.Println("[MAIN] From RM: Primary ID")
				elevator.PhysicalInfo.PrimaryID = newPrimaryId

				sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateMV_PhysicalInfo, updateOC_PhysicalInfo, updateTX_PhysicalInfo, updateHW_PhysicalInfo)
				updateRX_PrimaryID <- elevator.PhysicalInfo.PrimaryID
				processPairSnapshot(elevator, processPairsTx)

			case newAliveList := <-fromRM_AliveList:
				log.Println("[MAIN] From RM: New AliveList")
				isChanged := elevator.AliveList != newAliveList
				elevator.AliveList = newAliveList
				elevator.NumElevs = rolemanager.CountNumElevs(elevator.AliveList)

				if isChanged {
					updateOC_AliveList <- elevator.AliveList
					elev.SetAllLights(elevator.PhysicalInfo.LocalOrderTable, elevator.OrderTable, elevator.AliveList)
					processPairSnapshot(elevator, processPairsTx)
				}
			}

			//====================================================================== ACCEPTANCE TESTS

			if elevator.PhysicalInfo.Floor < 0 || elevator.PhysicalInfo.Floor >= elev.N_FLOORS {
				log.Fatalln("AT: Invalid floor index: ")
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

func processPairSnapshot(e elev.Elevator, tx chan<- elev.Elevator) {
	if tx == nil {
		return
	}

	select {
	case tx <- e:
	default:
	}
}

func WaitForInterrupt() {
	ch_sig := make(chan os.Signal, 1)
	signal.Notify(ch_sig, os.Interrupt, syscall.SIGTERM)
	<-ch_sig
}
