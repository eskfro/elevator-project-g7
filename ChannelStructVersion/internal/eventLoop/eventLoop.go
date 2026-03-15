package eventloop

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/movement"
	"elevator-project-g7/internal/network"
	ordercontrol "elevator-project-g7/internal/orderControl"
	rolemanager "elevator-project-g7/internal/roleManager"
	"log"
	"time"
)

func sendPhysicalInfoUpdate(info elev.ElevatorPhysicalInfo, channels ...chan<- elev.ElevatorPhysicalInfo) {
	for _, channel := range channels {
		select {
		case channel <- info:
		default:
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

	ch, rmInputs, rmOutputs, mvInputs, mvOutputs, ocInputs, ocOutputs, txInputs, rxInputs, rxOutputs := makeChannels()

	physicalInfoSubscribers := []chan<- elev.ElevatorPhysicalInfo{
		ch.rmLocalPhysicalInfo,
		ch.mvPhysicalInfo,
		ch.ocPhysicalInfo,
		ch.txPhysicalInfo,
	}
	// Start modules
	go movement.Movement(elevator, mvInputs, mvOutputs)
	go ordercontrol.OrderControl(elevator, ocInputs, ocOutputs)
	go rolemanager.RoleManager(elevator, rmInputs, rmOutputs)
	go network.TxHeartBeat(elevator, ports.HeartBeat, txInputs)
	go network.RxHeartBeat(ports.HeartBeat, elevator.PhysicalInfo.Id, rxOutputs)
	go network.TxOrderTable(elevator, ports.OrderTableP, txInputs)
	go network.RxOrderTable(elevator, ports.OrderTableP, rxInputs, rxOutputs)

	publishSnapshotPP(elevator, processPairsTx)

	lastFloorChange := time.Now()
	lastFloor := elevator.PhysicalInfo.Floor
	stuckTicker := time.NewTicker(500 * time.Millisecond)
	defer stuckTicker.Stop()

	// ========================================================================= PRINT DEBUGGING
	go func() {
		for range ticker_printElevator.C {
			uptime := time.Since(timeStart).Seconds()
			elev.PrintElevatorInfo(elevator, uptime)
			elev.PrintOrderTableSlice(elevator.OrderTable, elevator.PhysicalInfo.Id)
		}
	}()

	for {
		select {

		case <-stuckTicker.C:
			hasActiveOrders := ordercontrol.HasOrders(elevator.PhysicalInfo.LocalOrderTable)

			if hasActiveOrders && time.Since(lastFloorChange) > 10*time.Second {
				log.Fatalln("Stuck elevator, killing process pairs master")
			}
			// ================================================================= FROM HARDWARE

		case btn := <-fromIO_BtnPress:
			ch.ocButtonPress <- btn

		case obst := <-fromIO_Obstruction:
			log.Println("[MAIN] FromIO obs")
			elevator.PhysicalInfo.Obstructed = obst
			sendPhysicalInfoUpdate(elevator.PhysicalInfo, physicalInfoSubscribers...)
			publishSnapshotPP(elevator, processPairsTx)

		case floor := <-fromIO_Floor:
			log.Println("[MAIN] FromIO floor")

			if floor != lastFloor {
				lastFloorChange = time.Now()
				lastFloor = floor
			}

			elevator.PhysicalInfo.Floor = floor
			select {
			case ch.mvFloorArrival <- floor:
			default:
			}
			sendPhysicalInfoUpdate(elevator.PhysicalInfo, physicalInfoSubscribers...)
			publishSnapshotPP(elevator, processPairsTx)

		// ================================================================== FROM MOVEMENT

		case newLOT := <-ch.mvLocalOrderTable:
			log.Println("[MAIN] From MV: LocalOrderTable")
			elevator.PhysicalInfo.LocalOrderTable = newLOT
			sendPhysicalInfoUpdate(elevator.PhysicalInfo, physicalInfoSubscribers...)
			publishSnapshotPP(elevator, processPairsTx)

		case newMovement := <-ch.mvMovement:
			log.Println("[MAIN] From MV: Movement")
			elevator.PhysicalInfo.Movement = newMovement
			sendPhysicalInfoUpdate(elevator.PhysicalInfo, physicalInfoSubscribers...)
			publishSnapshotPP(elevator, processPairsTx)

		case newMotorDir := <-ch.mvMotorDir:
			log.Println("[MAIN] From MV: MotorDir")
			elevator.PhysicalInfo.MotorDir = newMotorDir
			sendPhysicalInfoUpdate(elevator.PhysicalInfo, physicalInfoSubscribers...)
			publishSnapshotPP(elevator, processPairsTx)

		case clr := <-ch.mvClearOrders:
			ch.ocClearOrders <- clr

		// ================================================================== FROM ORDERCONTROL

		case newOrderTable := <-ch.ocOrderTable:
			log.Println("[MAIN] From OC: OrderTable")

			newLOT := ordercontrol.OrderTableToLOT(newOrderTable, elevator.PhysicalInfo.Id)
			isLOTChanged := elevator.PhysicalInfo.LocalOrderTable != newLOT
			isOTChanged := elevator.OrderTable != newOrderTable

			elevator.OrderTable = newOrderTable
			elevator.PhysicalInfo.LocalOrderTable = newLOT

			if isLOTChanged {
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, physicalInfoSubscribers...)
				publishSnapshotPP(elevator, processPairsTx)
			}
			if isOTChanged {
				elev.SetAllLights(newLOT, newOrderTable, elevator.AliveList)
				publishSnapshotPP(elevator, processPairsTx)
			}
		case otp := <-ch.ocOutOrderTablePacket:
			ch.txOrderTablePacket <- otp

		// =================================================================== FROM ROLEMANAGER

		case v := <-ch.rmResetVersion:
			ch.rxResetversion <- v

		case newRole := <-ch.rmRole:
			log.Println("[MAIN] From RM: Role")
			isChanged := elevator.PhysicalInfo.Role != newRole
			elevator.PhysicalInfo.Role = newRole

			if isChanged {
				ch.txRole <- elevator.PhysicalInfo.Role
				ch.rxRole <- elevator.PhysicalInfo.Role
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, physicalInfoSubscribers...)
				publishSnapshotPP(elevator, processPairsTx)
			}

		case newPrimaryId := <-ch.rmPrimaryID:
			log.Println("[MAIN] From RM: Primary ID")
			isChanged := elevator.PhysicalInfo.PrimaryId != newPrimaryId
			elevator.PhysicalInfo.PrimaryId = newPrimaryId

			if isChanged {
				sendPhysicalInfoUpdate(elevator.PhysicalInfo, physicalInfoSubscribers...)
				ch.rxPrimaryId <- elevator.PhysicalInfo.PrimaryId
				publishSnapshotPP(elevator, processPairsTx)
			}

		case newAliveList := <-ch.rmAliveList:
			log.Println("[MAIN] From RM: New AliveList")
			isChanged := elevator.AliveList != newAliveList
			elevator.AliveList = newAliveList
			elevator.NumElevs = rolemanager.CountNumElevs(elevator.AliveList)

			if isChanged {
				ch.ocAliveList <- elevator.AliveList
				elev.SetAllLights(elevator.PhysicalInfo.LocalOrderTable, elevator.OrderTable, elevator.AliveList)
				publishSnapshotPP(elevator, processPairsTx)
			}

		// =================================================================== FROM NETWORK

		case otp := <-ch.rxOrderTablePacket:
			ch.ocInOrderTablePacket <- otp

		case info := <-ch.rxPhysicalInfo:
			ch.rmNetworkPhysicalInfo <- info
		}

		//===================== ACCEPTANCE TESTS ===============================

		if elevator.PhysicalInfo.Floor < 0 || elevator.PhysicalInfo.Floor >= elev.N_FLOORS {
			log.Fatalln("AT: Invalid floor index: ")
		}

	}
}
