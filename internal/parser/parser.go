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
		log.Fatalln("OS args missing - #1")
	}
	id, err := strconv.Atoi(args[1])
	if err != nil {
		log.Fatalln("OS args missing - #2")
	}
	port_HW, err := strconv.Atoi(args[2])
	if err != nil {
		log.Fatalln("OS args missing - #3")
	}
	port_HB, err := strconv.Atoi(args[3])
	if err != nil {
		log.Fatalln("OS args missing - #4")
	}
	port_OT, err := strconv.Atoi(args[4])
	if err != nil {
		log.Fatalln("OS args missing - #5")
	}

	return id, port_HW, port_HB, port_OT

}
