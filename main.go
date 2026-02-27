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
	ticker := time.NewTicker(2000 * time.Millisecond)
	ticker_AliveList := time.NewTicker(500 * time.Millisecond)
	timeStart := time.Now()
	defer ticker.Stop()

	//ticker := time.NewTicker(100 * time.Millisecond)
	ch_PrintTimer := make(chan bool)

	// POLLING
	ch_PollObstruction := make(chan bool)
	ch_PollFloorSensor := make(chan int)
	ch_PollButtonPress := make(chan elevio.ButtonEvent)

	// MODULE UPDATE CHANS
	ch_UpdateOC := make(chan elev.Elevator, 5)
	ch_UpdateMV := make(chan elev.Elevator, 5)

	// Trigger to Movement
	ch_FloorArrival := make(chan struct{})

	// Updates from Movement
	ch_LOTFromMV := make(chan elev.LocalOrderTable)
	ch_StateFromMV := make(chan elev.ElevatorMovement)
	ch_MotorDirFromMV := make(chan elevio.MotorDirection)

	// Trigger to OrderControl
	ch_OTPacketToOC := make(chan elev.OrderTablePacket)

	// Updates from OrderControl
	ch_ClearOrderFromOC := make(chan elevio.ButtonEvent)
	ch_LOTFromOC := make(chan elev.LocalOrderTable)

	// Trigger to RoleManager
	ch_toRM_HeartBeatId := make(chan int, 4)
	ch_updateRM_AliveList := make(chan elev.AliveList, 4)
	ch_updateRM_NumElevs := make(chan int, 4)
	ch_updateRM_PhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 4)

	// Updates from RoleManager
	ch_fromRM_Role := make(chan elev.ElevatorRole, 4)
	ch_fromRM_DeadElevId := make(chan int, 4)
	ch_fromRM_NumElevs := make(chan int, 4)

	// Trigger to Network
	ch_TxOrderTableP := make(chan elev.OrderTablePacket, 4)
	ch_UpdateTxMessage := make(chan elev.ElevatorPhysicalInfo, 4)

	// Updates from Network
	ch_RxOrderTableP := make(chan elev.OrderTablePacket, 4)
	ch_RxPhysicalInfo := make(chan elev.ElevatorPhysicalInfo, 4) //made this buffered

	// ============ GO MOVEMENT =======================

	go elevio.PollObstructionSwitch(ch_PollObstruction)
	go elevio.PollFloorSensor(ch_PollFloorSensor)
	go movement.GeneratePrintTimerEvents(ch_PrintTimer)
	go movement.Movement(ch_UpdateMV, ch_FloorArrival, ch_LOTFromMV, ch_StateFromMV, ch_MotorDirFromMV)

	// ============= GO ORDERCONTROL ===================

	go elevio.PollButtons(ch_PollButtonPress)
	go ordercontrol.OrderControl(ch_UpdateOC, ch_OTPacketToOC, ch_LOTFromOC)

	// ============= GO NETWORK ========================

	go network.Transmitter(port_OT, ch_TxOrderTableP)
	go network.Receiver(port_OT, ch_RxOrderTableP)
	go network.TxHeartBeat(port_HB, ch_UpdateTxMessage)
	go network.RxHeartBeat(port_HB, ch_RxPhysicalInfo, elevator.PhysicalInfo.Id)

	// ============= GO ROLE MANAGER ===================

	go rolemanager.RoleManager(ch_updateRM_AliveList, ch_updateRM_PhysicalInfo, ch_updateRM_NumElevs, ch_toRM_HeartBeatId, ch_fromRM_Role, ch_fromRM_DeadElevId, ch_fromRM_NumElevs)

	// Init stack elevators
	ch_UpdateOC <- elevator // TODO: defactor this channel
	ch_UpdateMV <- elevator // TODO: defactor this channel
	ch_updateRM_AliveList <- elevator.AliveList
	ch_updateRM_NumElevs <- elevator.NumElevs
	ch_updateRM_PhysicalInfo <- elevator.PhysicalInfo
	ch_UpdateTxMessage <- elevator.PhysicalInfo

	go func() {

		for {
			select {

			case <-ticker.C:
				uptime := int(time.Since(timeStart).Seconds())
				elev.PrintElevatorInfo(elevator, uptime)

			// ================================ MOVEMENT ============================
			// TO
			case obst := <-ch_PollObstruction:
				elevator.PhysicalInfo.Obstructed = obst

				ch_UpdateTxMessage <- elevator.PhysicalInfo
				ch_UpdateMV <- elevator

			case floor := <-ch_PollFloorSensor:

				elevator.PhysicalInfo.Floor = floor

				ch_UpdateTxMessage <- elevator.PhysicalInfo
				ch_UpdateMV <- elevator
				ch_UpdateOC <- elevator
				ch_FloorArrival <- struct{}{}

			// -----------------------------------------------------------------------
			// FROM
			case newLOT := <-ch_LOTFromMV:
				elevator.PhysicalInfo.LocalOrderTable = newLOT

				ch_UpdateTxMessage <- elevator.PhysicalInfo
				ch_UpdateMV <- elevator

			case newState := <-ch_StateFromMV:
				elevator.PhysicalInfo.State = newState

				ch_UpdateTxMessage <- elevator.PhysicalInfo
				ch_UpdateMV <- elevator
				ch_UpdateOC <- elevator

			case newMotorDir := <-ch_MotorDirFromMV:
				elevator.PhysicalInfo.MotorDir = newMotorDir

				ch_UpdateTxMessage <- elevator.PhysicalInfo
				ch_UpdateMV <- elevator
				ch_UpdateOC <- elevator

			// ================================ ORDERCONTROL ============================

			// TO
			case btnPress := <-ch_PollButtonPress:
				elevator.OrderTable[elevator.PhysicalInfo.Id][btnPress.Floor][btnPress.Button] = elev.OS_REQUESTED
				elevio.PrintButtonpress(btnPress)

				ch_UpdateOC <- elevator // TODO: make it such that only ordertable is updated here

			// -----------------------------------------------------------------------
			// FROM
			case order := <-ch_ClearOrderFromOC:
				elevator.OrderTable[elevator.PhysicalInfo.Id][order.Floor][order.Button] = elev.OS_CLEAR

				ch_UpdateOC <- elevator

			// ================================ ROLEMANAGER ============================

			// TO

			// -----------------------------------------------------------------------

			// FROM

			case <-ticker_AliveList.C:
				ch_updateRM_AliveList <- elevator.AliveList

			case deadElevId := <-ch_fromRM_DeadElevId:
				elevator.AliveList[deadElevId].Role = elev.ER_Dead

				ch_updateRM_AliveList <- elevator.AliveList

			case newRole := <-ch_fromRM_Role:
				elevator.PhysicalInfo.Role = newRole

				ch_UpdateTxMessage <- elevator.PhysicalInfo
				//ch_UpdateOC <- elevator // TODO: check for deadlocks

			case newNumElevs := <-ch_fromRM_NumElevs:
				elevator.NumElevs = newNumElevs

			// ================================ NETWORK ============================

			// TO

			// -----------------------------------------------------------------------

			// FROM

			//denne her er både From Network case og To OrderControl case
			case packet := <-ch_RxOrderTableP:

				if elevator.PhysicalInfo.Role == elev.ER_Primary {
					elevator.AllOrderTables[packet.Id] = packet.OrderTable

					ch_UpdateOC <- elevator

				}
				ch_OTPacketToOC <- packet

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
