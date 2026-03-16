package main

import (
	"elevator-project-g7/internal/parser"
	processpairs "elevator-project-g7/internal/processPairs"
	"os"
	"os/signal"
	"syscall"
)

const sim bool = false

func WaitForInterrupt() {
	ch_sig := make(chan os.Signal, 1)
	signal.Notify(ch_sig, os.Interrupt, syscall.SIGTERM)
	<-ch_sig
}

func main() {
	id, ports, ppRole := parser.ParseOsArgs(os.Args, sim)

	processpairs.Start(
		id,
		ports,
		ppRole,
	)

	WaitForInterrupt()
}
