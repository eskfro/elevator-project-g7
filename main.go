package main

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	eventloop "elevator-project-g7/internal/eventLoop"
	"elevator-project-g7/internal/network"
	"elevator-project-g7/internal/parser"
	"fmt"
	"os"
)

const sim bool = false

func main() {
	id, ports, ppMaster := parser.ParseOsArgs(os.Args, sim)

	//processpairs.Start(id, ports, ppMaster)
	fmt.Println(ppMaster)

	// INIT
	elevio.InitPhysicalElevator("localhost", ports.Hardware, elev.N_FLOORS)
	elevator := elev.CreateElevator(id, ports.Hardware, network.GetLocalIP())
	elev.PrintElevatorInit(id, ports.Hardware)
	elev.PrintElevatorInfo(elevator, 0)

	fromIO_BtnPress, fromIO_Floor, fromIO_Obstruction := elevio.Inputs()

	eventloop.Start(elevator, ports, fromIO_BtnPress, fromIO_Floor, fromIO_Obstruction)

}
