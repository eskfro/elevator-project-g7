package main

import (
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/movement"
	"elevator-project-g7/internal/parser"
	"fmt"
	"os"
)

// IMPORTANT
// Public functions start with uppercase
//  -> Functions that start with lowercase are private to the package
// Packages need to start with lowecase

func main() {
	// Parse args
	id, port, err := parser.ParseOsArgs(os.Args)
	if err != nil {
		fmt.Println("Failed to parse os args")
		return
	}

	// init things
	movement.InitPhysicalElevatorToFloor("localhost", port, 0) //Move to floor 0
	e := movement.CreateElevator(id, port)
	fmt.Printf("elevator starting | id = %d | port = %d\n", id, port)

	// channels
	chanStopButton := make(chan bool)

	go elevio.PollStopButton(chanStopButton)
	go elevio.PrintStopButton(chanStopButton)

	for {
	}

}
