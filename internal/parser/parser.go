package parser

import (
	"log"
	"strconv"
)

func ParseOsArgs(args []string, sim bool) (int, int) {
	/*
		Parser for OS-args when running program from cmd line
	*/

	if sim {
		return 1, 15657
	}
	if len(args) < 3 {
		log.Fatalln("OS args missing - #1")
	}
	id, err := strconv.Atoi(args[1])
	if err != nil {
		log.Fatalln("OS args missing - #2")
	}
	port, err := strconv.Atoi(args[2])
	if err != nil {
		log.Fatalln("OS args missing - #3")
	}

	return id, port

}
