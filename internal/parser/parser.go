package parser

import (
	"errors"
	"strconv"
)

func ParseOsArgs(args []string) (int, int, error) {

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
