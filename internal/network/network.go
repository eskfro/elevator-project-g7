package network

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/network/localip"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"
)

func PrintMessageFromNetwork() {
	fmt.Println("This is a message from the Network module")
	printLocalIP()
}

func TxHeartBeat(port_hb int, ch_UpdateTxMessage chan elev.ElevatorPhysicalInfo) {

	address := "255.255.255.255" + ":" + strconv.Itoa(port_hb)

	// Establish udp "connection"
	conn, err := net.Dial("udp", address)
	if err != nil {
		log.Printf("TxHeartBeat conn failed! port_hb = %d\n", port_hb)
		log.Printf("TxHeartBeat failed: %v\n", err)
	}
	defer conn.Close()

	var message elev.ElevatorPhysicalInfo

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case message = <-ch_UpdateTxMessage:

		case <-ticker.C:

			data, err := json.Marshal(message)
			if err != nil {
				log.Println("JSON Marshal error:", err)
				continue
			}

			_, err = conn.Write(data)
			if err != nil {
				log.Println("Error sending message: ", err)
				continue
			}
		}
	}
}

func RxHeartBeat(port_hb int, ch_RxPhysicalInfo chan elev.ElevatorPhysicalInfo) {
	addr := net.UDPAddr{
		IP:   nil,
		Port: port_hb,
	}

	var conn *net.UDPConn
	var err error

	// Code ensuring the startup logic is patient.
	// Don't just fail if the port is busy, loop until it's free.
	for {
		conn, err = net.ListenUDP("udp", &addr)
		if err == nil {
			log.Println("Established Connection :)")
			break
		}
		log.Printf("Port busy")
		time.Sleep(100 * time.Millisecond)
	}
	defer conn.Close()

	buf := make([]byte, 2048)

	for {

		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		var recievedInfo elev.ElevatorPhysicalInfo

		err = json.Unmarshal(buf[:n], &recievedInfo)
		if err != nil {
			log.Printf("JSON unmarshal failed: %v\n", err)
			continue
		}

		ch_RxPhysicalInfo <- recievedInfo
	}
}

func printLocalIP() {
	addr, _ := localip.LocalIP()
	fmt.Println(addr)
}
