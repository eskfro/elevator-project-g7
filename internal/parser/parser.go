package parser

import (
	"log"
	"strconv"
)

func ParseOsArgs(args []string, sim bool) (uint16, uint16) {
	/*
		Parser for OS-args when running program from cmd line
	*/

	if sim {
		return 1, 15657
	}
	if len(args) < 3 {
		log.Fatalln("OS args missing - #1")
	}
	id, err := strconv.ParseUint(args[1], 10, 16)
	if err != nil {
		log.Fatalln("OS args missing - #2")
	}
	port, err := strconv.ParseUint(args[2], 10, 16)
	if err != nil {
		log.Fatalln("OS args missing - #3")
	}

	return uint16(id), uint16(port)

}
