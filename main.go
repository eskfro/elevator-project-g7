package main

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/movement"
	"elevator-project-g7/internal/parser"
	"fmt"
	"os"
)

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

	// init things
	elev.InitPhysicalElevator("localhost", port)
	//elev.CreateElevator(id, port)
	fmt.Printf("elevator starting | id = %d | port = %d\n", id, port)

	// gg
	movement.Start(id, port)

	for {
	}

}
