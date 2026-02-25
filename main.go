package main

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/movement"
	network "elevator-project-g7/internal/network/bcast"
	"elevator-project-g7/internal/ordercontrol"
	"elevator-project-g7/internal/parser"
	"elevator-project-g7/internal/rolemanager"
	"os"
	"os/signal"
	"syscall"
)

func WaitForInterrupt() {
	ch_sig := make(chan os.Signal, 1)
	signal.Notify(ch_sig, os.Interrupt, syscall.SIGTERM)
	<-ch_sig
}

const sim bool = true

func main() {
	// PARSE ARGS
	id, port := parser.ParseOsArgs(os.Args, sim)

	// INIT
	elevio.InitPhysicalElevator("localhost", port, elev.N_FLOORS)
	elevator := elev.CreateElevator(id, port)
	elev.PrintElevatorInit(id, port)

	//ticker := time.NewTicker(100 * time.Millisecond)
	ch_PrintTimer := make(chan bool)

	// POLLING
	ch_PollObstruction := make(chan bool)
	ch_PollFloorSensor := make(chan int)
	ch_PollButtonPress := make(chan elevio.ButtonEvent)

	// MODULE UPDATE CHANS
	ch_UpdateOC := make(chan elev.Elevator)
	ch_UpdateMV := make(chan elev.Elevator)
	ch_UpdateRM := make(chan elev.Elevator)

	ch_RcvAliveList := make(chan elev.AliveList)

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

	// Updates from RoleManager
	ch_RoleUpdateFromRM := make(chan elev.ElevatorRole)

	// Trigger to Network
	ch_TxOrderTableP := make(chan elev.OrderTablePacket)
	ch_TxAliveListP := make(chan elev.AliveListPacket)

	// Updates from Network
	ch_RxOrderTableP := make(chan elev.OrderTablePacket)
	ch_RxAliveListP := make(chan elev.AliveListPacket)

	// ============ GO MOVEMENT =======================

	go elevio.PollObstructionSwitch(ch_PollObstruction)
	go elevio.PollFloorSensor(ch_PollFloorSensor)
	go movement.GeneratePrintTimerEvents(ch_PrintTimer)
	go movement.Movement(ch_UpdateMV, ch_PrintTimer, ch_FloorArrival, ch_LOTFromMV, ch_StateFromMV, ch_MotorDirFromMV)

	// ============= GO ORDERCONTROL ===================

	go elevio.PollButtons(ch_PollButtonPress)
	go ordercontrol.OrderControl(ch_UpdateOC, ch_OTPacketToOC, ch_LOTFromOC)

	// ============= GO ROLE MANAGER ===================

	go rolemanager.RoleManager(ch_UpdateRM, ch_RcvAliveList, ch_RoleUpdateFromRM)

	// ============= GO NETWORK ========================

	go network.Transmitter(port, ch_TxOrderTableP, ch_TxAliveListP)
	go network.Receiver(port, ch_RxOrderTableP, ch_RxAliveListP)

	// Init stack elevators
	ch_UpdateOC <- elevator
	ch_UpdateMV <- elevator
	ch_UpdateRM <- elevator

	go func() {

		for {
			select {

			// ================================ MOVEMENT ============================
			// TO
			case obst := <-ch_PollObstruction:
				elevator.PhysicalInfo.Obstructed = obst

			case floor := <-ch_PollFloorSensor:
				elevator.PhysicalInfo.Floor = floor
				ch_UpdateMV <- elevator
				ch_FloorArrival <- struct{}{}

			// -----------------------------------------------------------------------
			// FROM
			case newLOT := <-ch_LOTFromMV:
				elevator.PhysicalInfo.LocalOrderTable = newLOT

			case newState := <-ch_StateFromMV:
				elevator.PhysicalInfo.State = newState

			case newMotorDir := <-ch_MotorDirFromMV:
				elevator.PhysicalInfo.MotorDir = newMotorDir

			// ================================ ORDERCONTROL ============================

			// TO
			case btnPress := <-ch_PollButtonPress:
				elevator.OrderTable[elevator.PhysicalInfo.Id][btnPress.Floor][btnPress.Button] = elev.OS_REQUESTED
				elevio.PrintButtonpress(btnPress)
			// -----------------------------------------------------------------------
			// FROM
			case order := <-ch_ClearOrderFromOC:
				elevator.OrderTable[elevator.PhysicalInfo.Id][order.Floor][order.Button] = elev.OS_CLEAR

			// ================================ ROLEMANAGER ============================

			// TO

			// -----------------------------------------------------------------------

			// FROM

			case newRole := <-ch_RoleUpdateFromRM:
				elevator.PhysicalInfo.Role = newRole

			// ================================ NETWORK ============================

			// TO

			// -----------------------------------------------------------------------

			// FROM

			//denne her er både From Network case og To OrderControl case
			case packet := <-ch_RxOrderTableP:

				switch elevator.PhysicalInfo.Role {

				case elev.ER_Init:

					// TODO

				case elev.ER_Backup:

					// message not from primary
					if packet.Id != elevator.PhysicalInfo.PrimaryId {
						break
					}
					elevator.AllOrderTables[packet.Id] = packet.OrderTable
					ch_UpdateOC <- elevator
					ch_OTPacketToOC <- packet

				case elev.ER_Primary:

					// message from self
					if packet.Id == elevator.PhysicalInfo.Id {
						break
					}
					elevator.AllOrderTables[packet.Id] = packet.OrderTable
					ch_UpdateOC <- elevator
					ch_OTPacketToOC <- packet

				}

			case packet := <-ch_RxAliveListP:

				switch elevator.PhysicalInfo.Role {

				case elev.ER_Init:

					// TODO

				case elev.ER_Backup:

					// message not from primary
					if packet.Id != elevator.PhysicalInfo.PrimaryId {
						break
					}
					elevator.AliveList = packet.AliveList

				case elev.ER_Primary:

					// message from self
					if packet.Id == elevator.PhysicalInfo.Id {
						break
					}

				}

				if packet.Id != elevator.PhysicalInfo.Id && packet.Id == elevator.PhysicalInfo.PrimaryId {
					elevator.AliveList = packet.AliveList
				}

			}

			// Update all local elevator objects after any case
			ch_UpdateOC <- elevator
			ch_UpdateMV <- elevator
			ch_UpdateRM <- elevator
		}
	}()

	WaitForInterrupt()

	// TODO: se log.txt

}
