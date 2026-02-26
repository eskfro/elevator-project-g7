package parser

import (
	"log"
	"strconv"
)

func ParseOsArgs(args []string, sim bool) (int, int, int, int) {
	/*
		Parser for OS-args when running program from cmd line
		Returns Id, HardwarePort, HeartBeatPort, OrderTablePort
	*/

	if sim {
		return 0, 15657, 16767, 16668
	}
	if len(args) < 5 {
		log.Fatalln("OS args missing! (1)")
	}

	id, err0 := strconv.Atoi(args[1])
	port_HW, err1 := strconv.Atoi(args[2])
	port_HB, err2 := strconv.Atoi(args[3])
	port_OT, err3 := strconv.Atoi(args[4])

	if err0 != nil || err1 != nil || err2 != nil || err3 != nil {
		log.Fatalln("OS args missing! (2)")
	}

	return id, port_HW, port_HB, port_OT

}
