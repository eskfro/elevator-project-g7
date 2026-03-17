package main

import (
	"elevator-project-g7/internal/parser"
	processpairs "elevator-project-g7/internal/processPairs"
	"os"
)

const sim bool = false

func main() {
	id, ports, ppRole := parser.ParseOsArgs(os.Args, sim)

	processpairs.Start(
		id,
		ports,
		ppRole,
	)

}
