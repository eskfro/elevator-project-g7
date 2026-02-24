package main

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/movement"
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

const sim bool = true

func main() {
	// Parse args
	id, port := parser.ParseOsArgs(os.Args, sim)

	// Init The Elevator Core
	elevio.InitPhysicalElevator("localhost", port, elev.N_FLOORS)
	elevator := elev.CreateElevator(id, port)
	elev.PrintElevatorInit(id, port)
	ticker := time.NewTicker(100 * time.Millisecond)

	// TODO: kanskje ikke bruke struct nå lenger fordi tror dette skal være i main

	ch_PrintTimer := make(chan bool)
	ch_PollObstruction := make(chan bool)
	ch_PollFloorSensor := make(chan int)
	ch_PollButtonPress := make(chan elevio.ButtonEvent)

	ch_RcvOrderTable := make(chan elev.OrderTable)
	ch_RcvAliveList := make(chan elev.AliveList)
	ch_BcastTick := ticker.C

	// Channels for updating local elevators in modules
	ch_UpdateOC := make(chan elev.Elevator)
	ch_UpdateMV := make(chan elev.Elevator)
	ch_UpdateRM := make(chan elev.Elevator)

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

	// Updates from Network
	ch_RcvOTPacket := make(chan elev.OrderTablePacket)

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
	go network.RecieveInfo(ch_RcvOrderTable, ch_RcvAliveList)
	go network.SendInfo(ch_BcastTick, ch_xxxOrderTable, ch_xxxAlivelist)

	// Init stack elevators
	ch_UpdateOC <- elevator
	ch_UpdateMV <- elevator
	ch_UpdateRM <- elevator

	go func() {

		for {
			select {

			// To Movement cases
			case obst := <-ch_PollObstruction:
				elevator.PhysicalInfo.Obstructed = obst

			case floor := <-ch_PollFloorSensor:
				elevator.PhysicalInfo.Floor = floor
				ch_UpdateMV <- elevator
				ch_FloorArrival <- struct{}{}

			//From Movement cases
			case newLOT := <-ch_LOTFromMV:
				elevator.PhysicalInfo.LocalOrderTable = newLOT

			case newState := <-ch_StateFromMV:
				elevator.PhysicalInfo.State = newState

			case newMotorDir := <-ch_MotorDirFromMV:
				elevator.PhysicalInfo.MotorDir = newMotorDir

			// To OrderControl cases
			case btnPress := <-ch_PollButtonPress:
				elevator.OrderTable[elevator.PhysicalInfo.Id][btnPress.Floor][btnPress.Button] = elev.OS_REQUESTED
				elevio.PrintButtonpress(btnPress)

			// From OrderControl cases
			case order := <-ch_ClearOrderFromOC:
				elevator.OrderTable[elevator.PhysicalInfo.Id][order.Floor][order.Button] = elev.OS_CLEAR

			// To Rolemanager cases

			// From Rolemanager cases
			case newRole := <-ch_RoleUpdateFromRM:
				elevator.PhysicalInfo.Role = newRole

			// To Network cases

			// From Network cases

			//denne her er både From Network case og To OrderControl case
			case packet := <-ch_RcvOTPacket:
				elevator.AllOrderTables[packet.Id] = packet.OrderTable
				ch_UpdateOC <- elevator
				ch_OTPacketToOC <- packet
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
