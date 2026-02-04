package network

import (
	"elevator-project-g7/internal/network/localip"
	"fmt"
)

func PrintMessageFromNetwork() {
	fmt.Println("This is a message from the Network module")
	printLocalIP()
}

func printLocalIP() {
	addr, _ := localip.LocalIP()
	fmt.Println(addr)
}
