package eventloop

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
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
	fromRM_PrimaryID := make(chan int, 20)
	fromRM_AliveList := make(chan elev.AliveList, 20)
	fromRM_ResetVersion := make(chan int, 10)

	// Network transmitter
	updateTX_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 20)
	updateTX_OTP := make(chan elev.OrderTablePacket, 100)
	updateTX_Role := make(chan elev.ElevatorRole, 5)

	// Network reciever
	updateRX_Role := make(chan elev.ElevatorRole, 5)
	updateRX_PrimaryID := make(chan int, 5)
	fromRX_OrderTableP := make(chan elev.OrderTablePacket, 100)
	fromRX_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 100)

	// Hardware
	fromHW_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 100)
	updateHW_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 100)

	// Start modules
	go movement.Movement(elevator, updateMV_PhysicalInfo, fromMV_LOT, fromMV_Movement, fromMV_MotorDir, fromMV_ClearOrders, toMV_FloorArrival)
	go ordercontrol.OrderControl(elevator, updateOC_PhysicalInfo, updateOC_AliveList, fromRX_OrderTableP, fromMV_ClearOrders, fromIO_BtnPress, fromOC_OrderTable, updateTX_OTP)
	go rolemanager.RoleManager(elevator, fromRX_PhysicalInfo, updateRM_PhysicalInfo, fromRM_Role, fromRM_PrimaryID, fromRM_AliveList, fromRM_ResetVersion)
	go network.TxHeartBeat(elevator, ports.HeartBeat, updateTX_PhysicalInfo)
	go network.RxHeartBeat(ports.HeartBeat, fromRX_PhysicalInfo, elevator.PhysicalInfo.ID)
	go network.TxOrderTable(elevator, ports.OrderTableP, updateTX_OTP, updateTX_Role)
	go network.RxOrderTable(elevator, ports.OrderTableP, updateRX_Role, updateRX_PrimaryID, fromRM_ResetVersion, fromRX_OrderTableP)

	publishSnapshotPP(elevator, processPairsTx)

	// ==================================================================== PRINT ELEVATOR
	go func(initElev elev.Elevator, fromIO_Obstruction <-chan bool, fromIO_Floor <-chan int,
		fromHW_PhysicalInfo chan<- elev.ElevatorPhysicalInfo, toMV_FloorArrival chan<- int, updateHW_PhysicalInfo <-chan elev.ElevatorPhysicalInfo,
	) {

		// Local
		physicalInfo := initElev.PhysicalInfo

		for {
			select {

			case physicalInfo = <-updateHW_PhysicalInfo:

			case obst := <-fromIO_Obstruction:
				log.Println("[MAIN] FromIO obs")
				physicalInfo.Obstructed = obst
				fromHW_PhysicalInfo <- physicalInfo

			case floor := <-fromIO_Floor:
				log.Println("[MAIN] FromIO floor")
				physicalInfo.Floor = floor
				toMV_FloorArrival <- floor
				fromHW_PhysicalInfo <- physicalInfo

			}
		}
	}(elevator, fromIO_Obstruction, fromIO_Floor, fromHW_PhysicalInfo, toMV_FloorArrival, updateHW_PhysicalInfo)

	func() {

		for {
			select {

			case newPhysicalInfo := <-fromHW_PhysicalInfo:
				log.Println("[eventLoop] From HW PhysicalInfo ")
				elevator.PhysicalInfo = newPhysicalInfo
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateRM_PhysicalInfo, updateOC_PhysicalInfo, updateTX_PhysicalInfo, updateMV_PhysicalInfo)
				publishSnapshotPP(elevator, processPairsTx)

			case <-ticker_printElevator.C:
				uptime := time.Since(timeStart).Seconds()
				elev.PrintElevatorInfo(elevator, uptime)
				elev.PrintOrderTableSlice(elevator.OrderTable, elevator.PhysicalInfo.ID)

			// ================================================================== FROM MOVEMENT
			case newLOT := <-fromMV_LOT:
				log.Println("[MAIN] From MV: LocalOrderTable")
				elevator.PhysicalInfo.LocalOrderTable = newLOT
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateRM_PhysicalInfo, updateOC_PhysicalInfo, updateTX_PhysicalInfo, updateHW_PhysicalInfo)
				publishSnapshotPP(elevator, processPairsTx)

			case newMovement := <-fromMV_Movement:
				log.Println("[MAIN] From MV: Movement")
				elevator.PhysicalInfo.Movement = newMovement
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateRM_PhysicalInfo, updateOC_PhysicalInfo, updateTX_PhysicalInfo, updateHW_PhysicalInfo)
				publishSnapshotPP(elevator, processPairsTx)

			case newMotorDir := <-fromMV_MotorDir:
				log.Println("[MAIN] From MV: MotorDir")
				elevator.PhysicalInfo.MotorDir = newMotorDir
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateRM_PhysicalInfo, updateOC_PhysicalInfo, updateTX_PhysicalInfo, updateHW_PhysicalInfo)
				publishSnapshotPP(elevator, processPairsTx)

			// ================================================================== FROM ORDERCONTROL

			case newOrderTable := <-fromOC_OrderTable:
				log.Println("[MAIN] From OC: OrderTable")
				isLOTChanged := elevator.PhysicalInfo.LocalOrderTable != ordercontrol.OrderTableToLOT(newOrderTable, elevator.PhysicalInfo.ID)
				isOTChanged := elevator.OrderTable != newOrderTable

				elevator.OrderTable = newOrderTable
				elevator.PhysicalInfo.LocalOrderTable = ordercontrol.OrderTableToLOT(elevator.OrderTable, elevator.PhysicalInfo.ID)

				if isLOTChanged {
					sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateMV_PhysicalInfo, updateHW_PhysicalInfo)
					publishSnapshotPP(elevator, processPairsTx)
				}
				if isOTChanged {
					elev.SetAllLights(elevator.PhysicalInfo.LocalOrderTable, elevator.OrderTable, elevator.AliveList)
					publishSnapshotPP(elevator, processPairsTx)
				}

			// =================================================================== FROM ROLEMANAGER

			case newRole := <-fromRM_Role:
				log.Println("[MAIN] From RM: Role")
				isChanged := elevator.PhysicalInfo.Role != newRole
				elevator.PhysicalInfo.Role = newRole

				if isChanged {
					updateTX_Role <- elevator.PhysicalInfo.Role
					updateRX_Role <- elevator.PhysicalInfo.Role
					sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateMV_PhysicalInfo, updateOC_PhysicalInfo, updateTX_PhysicalInfo, updateHW_PhysicalInfo)
					publishSnapshotPP(elevator, processPairsTx)
				}

			case newPrimaryId := <-fromRM_PrimaryID:
				log.Println("[MAIN] From RM: Primary ID")
				isChanged := elevator.PhysicalInfo.PrimaryID != newPrimaryId
				elevator.PhysicalInfo.PrimaryID = newPrimaryId

				if isChanged {
					sendPhysicalInfoUpdate(elevator.PhysicalInfo, updateMV_PhysicalInfo, updateOC_PhysicalInfo, updateTX_PhysicalInfo, updateHW_PhysicalInfo)
					updateRX_PrimaryID <- elevator.PhysicalInfo.PrimaryID
					publishSnapshotPP(elevator, processPairsTx)
				}

			case newAliveList := <-fromRM_AliveList:
				log.Println("[MAIN] From RM: New AliveList")
				isChanged := elevator.AliveList != newAliveList
				elevator.AliveList = newAliveList
				elevator.NumElevs = rolemanager.CountNumElevs(elevator.AliveList)

				if isChanged {
					updateOC_AliveList <- elevator.AliveList
					elev.SetAllLights(elevator.PhysicalInfo.LocalOrderTable, elevator.OrderTable, elevator.AliveList)
					publishSnapshotPP(elevator, processPairsTx)
				}
			}

			//===================== ACCEPTANCE TESTS ===============================

			// TODO: Denne må flyttes herifra, eventloopen skal bare inneholde events mellom moduler

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
func publishSnapshotPP(e elev.Elevator, tx chan<- elev.Elevator) {
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
