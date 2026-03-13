package main

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/eventloop"
	"elevator-project-g7/internal/network"
	"elevator-project-g7/internal/parser"
	"os"
)

const sim bool = false

func main() {
	id, ports := parser.ParseOsArgs(os.Args, sim)

	// INIT
	elevio.InitPhysicalElevator("localhost", ports.Hardware, elev.N_FLOORS)
	elevator := elev.CreateElevator(id, ports.Hardware, network.GetLocalIP())
	elev.PrintElevatorInit(id, ports.Hardware)
	elev.PrintElevatorInfo(elevator, 0)

	fromIO_BtnPress, fromIO_Floor, fromIO_Obstruction := elevio.Inputs()

	eventloop.Start(elevator, ports, fromIO_BtnPress, fromIO_Floor, fromIO_Obstruction)

}

// TODO: Av og til når en ny heis kommer på nettverket så fungerer ikke ordre får en cab trykk på sin egen.
