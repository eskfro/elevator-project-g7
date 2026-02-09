package parser

import (
	"errors"
	"strconv"
)

func ParseOsArgs(args []string) (int, int, error) {
	/*
		Parser for OS-args when running program from cmd line

		Example:
		./out 1 15657
		id = 1
		port = 15657

	*/

	if len(args) < 3 {
		return 0, 0, errors.New("Missing arguments")
	}

	id, err := strconv.Atoi(args[1])
	if err != nil {
		return 0, 0, errors.New("Int conversion failed (1)")
	}
	port, err := strconv.Atoi(args[2])
	if err != nil {
		return 0, 0, errors.New("Int conversion failed (2)")
	}

	return id, port, nil

}
