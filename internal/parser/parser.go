package parser

import (
	"elevator-project-g7/internal/network"
	"log"
	"strconv"
)

func ParseOsArgs(args []string, sim bool) (int, network.Ports, string) {
	/*
		Parser for OS-args when running program from cmd line
		Returns Id, Ports, ProcessPairRole
	*/

	if sim {
		return 0, network.Ports{Hardware: 15657, OrderTableP: 16767, HeartBeat: 16668}, "1"
	}
	if len(args) < 6 {
		log.Fatalln("OS args missing! (1)")
	}

	id, err0 := strconv.Atoi(args[1])
	port_HW, err1 := strconv.Atoi(args[2])
	port_HB, err2 := strconv.Atoi(args[3])
	port_OT, err3 := strconv.Atoi(args[4])
	ppRole := args[5]

	if err0 != nil || err1 != nil || err2 != nil || err3 != nil {
		log.Fatalln("OS args missing! (2)")
	}

	return id, network.Ports{Hardware: port_HW, OrderTableP: port_OT, HeartBeat: port_HB}, ppRole

	// ppRole = "1" -> Programmet starter som en master
	// ppRole = "0" -> Programmet starter som slave, denne er blitt kalt fra en master.

}
