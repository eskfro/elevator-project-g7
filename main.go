package main

import (
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/parser"
	processpairs "elevator-project-g7/internal/processPairs"
	"os"
)

const sim bool = false

func main() {
	id, ports, ppRole := parser.ParseOsArgs(os.Args, sim)

	fromIO_BtnPress, fromIO_Floor, fromIO_Obstruction := elevio.Inputs()

	processpairs.Start(
		id,
		ports,
		ppRole,
		fromIO_BtnPress,
		fromIO_Floor,
		fromIO_Obstruction,
	)
}
