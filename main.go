package main

import (
	"elevator-project-g7/internal/elevator"
	"elevator-project-g7/internal/parser"
	"fmt"
	"os"
)

// IMPORTANT
// Public functions start with uppercase
//  -> Functions that start with lowercase are private to the package
// Packages need to start with lowecase

func main() {
	// Extract os args
	id, port, err := parser.ParseOsArgs(os.Args)
	if err != nil {
		fmt.Println("Failed to parse os args")
		return
	}

	// Things
	elevator.InitPhysicalElevator("localhost", port, 2)
	elevator.CreateElevator(id, port)
	fmt.Printf("elevator starting | id = %d | port = %d\n", id, port)

}
