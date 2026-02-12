package main

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/movement"
	"elevator-project-g7/internal/ordercontrol"
	"elevator-project-g7/internal/parser"
	"elevator-project-g7/internal/rolemanager"
	"fmt"
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

// IMPORTANT
// Public functions start with uppercase
//  -> Functions that start with lowercase are private to the package
// Packages need to start with lowecase

const sim bool = true

func main() {
	// Parse args
	var id, port int
	var err error

	if sim {
		id, port, err = 1, 15657, nil
	} else {
		id, port, err = parser.ParseOsArgs(os.Args)
		if err != nil {
			fmt.Println("Failed to parse os args")
			return
		}
	}

	// Init The Elevator Core
	elev.InitPhysicalElevator("localhost", port)
	E := elev.CreateElevator(id, port)
	elev.PrintElevatorInit(id, port)
	ticker := time.NewTicker(100 * time.Millisecond)

	// TODO: kanskje ikke bruke struct nå lenger fordi tror dette skal være i main
	ch_Movement := movement.Channels{
		PrintTimer:   make(chan bool),
		Obstruction:  make(chan bool),
		FloorArrival: make(chan int),
	}

	ch_OrderControl := ordercontrol.Channels{
		ButtonPress:      make(chan elevio.ButtonEvent),
		ClearOrder:       make(chan elev.Order),     //Primary
		RequestConfirmed: make(chan elev.Order),     //Primary
		RcvBcast:         make(chan elev.WorldView), //Primary		 //TODO: WorldView is not a type.
		NewOrderRequest:  make(chan elev.Order),     //Primary
		MsgFromPrimary:   make(chan [elev.N_MAX_ELEVS][elev.N_FLOORS][elev.N_BUTTONS]elev.OrderStatus),
		RoleUpdate:       make(chan elev.ElevatorRole),
		BroadcastWorldView: ticker.C,
	}

	// Her må vi gå bort fra pekere fordi det blir jo race conditions :)
	// Tenker sender vel inn en channel for elevator og diverse.

	// ============ GO MOVEMENT =======================

	go elevio.PollObstructionSwitch(ch_Movement.Obstruction)
	go elevio.PollFloorSensor(ch_Movement.FloorArrival)
	go movement.GeneratePrintTimerEvents(ch_Movement.PrintTimer)
	go movement.HandleEvents(&E.PhysicalInfo, &E.WorldView.LocalOrderTable, &ch_Movement)

	// ============= GO CONTROL =====================

	go elevio.PollButtons(ch_OrderControl.ButtonPress)
	go rolemanager.PollRoleUpdate(ch_OrderControl.RoleUpdate, &E.Role)
	go ordercontrol.OrderControl(&E.WorldView, &E.AllWorldViews, ch_OrderControl, &E.NumElevs, &E.Id, &E.Role)

	WaitForInterrupt()

	// TODO: se log.txt

}
