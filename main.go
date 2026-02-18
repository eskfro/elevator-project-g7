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
	elev.InitPhysicalElevator("localhost", port)
	E := elev.CreateElevator(id, port)
	elev.PrintElevatorInit(id, port)
	ticker := time.NewTicker(100 * time.Millisecond)

	// TODO: kanskje ikke bruke struct nå lenger fordi tror dette skal være i main

	ch_Elevator := 			make(chan elev.Elevator)
	ch_PrintTimer :=  		make(chan bool)
	ch_Obstruction :=  		make(chan bool)
	ch_FloorArrival := 		make(chan int)
	ch_ButtonPress := 		make(chan elevio.ButtonEvent)
	ch_RcvOrderTable :=     make(chan elev.OrderTable)
	ch_RcvAliveList := 		make(chan elev.AliveList)
	ch_RoleUpdate :=       	make(chan elev.ElevatorRole)
	ch_BcastTick := ticker.C

	ch_UpdateOC := 			make(chan elev.Elevator)
	ch_UpdateMV := 			make(chan elev.Elevator)
	ch_UpdateRM := 			make(chan elev.Elevator)



	// Her må vi gå bort fra pekere fordi det blir jo race conditions :)
	// Tenker sender vel inn en channel for elevator og diverse.

	// ============ GO MOVEMENT =======================

	go elevio.PollObstructionSwitch(ch_Obstruction)
	go elevio.PollFloorSensor(ch_FloorArrival)
	go movement.GeneratePrintTimerEvents(ch_PrintTimer)
	go movement.Movement(ch_UpdateMV)

	// ============= GO ORDERCONTROL ===================

	go elevio.PollButtons(ch_ButtonPress)
	go rolemanager.PollRoleUpdate(ch_RoleUpdate, &E.Role)
	go ordercontrol.OrderControl(ch_UpdateOC, ch_RcvOrderTable)

	// ============= GO ROLE MANAGER ===================
	go rolemanager.RoleManager(ch_RcvAliveList, ch_UpdateRM)

	// ============= GO NETWORK ========================
	go network.RecieveInfo(ch_RcvOrderTable, ch_RcvAliveList)
	go network.SendInfo(ch_BcastTick, ch_xxxOrderTable, ch_xxxAlivelist)


	go func () {

		for { 
			select {
			case obst := <-ch_Obstruction:
				elevator.PhysicalInfo.Obstructed = obst

			case ...:

			case ...:

			}
			ch_UpdateOC <- E
			ch_UpdateMV <- E
			ch_UpdateRM <- E
		}
	}()



	WaitForInterrupt()

	// TODO: se log.txt

}
