package network

import (
	"context"
	"elevator-project-g7/internal/elev"
	"encoding/json"
	"log"
	"net"
	"strconv"
	"syscall"
	"time"
)

type Ports struct {
	Hardware    int
	OrderTableP int
	HeartBeat   int
}

// UDP config, denne har jeg testet før så den skal fungere
var lc = net.ListenConfig{
	Control: func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			// 1. Allow multiple processes to use the port
			syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			// 2. Explicitly permit broadcasting from this socket
			syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
		})
	},
}

func TxHeartBeat(
	initElev elev.Elevator,
	port_hb int,
	ch_updateTxOT chan elev.OrderTable,
	ch_updateTxPhysicalInfo chan elev.ElevatorPhysicalInfo) {

	initOTP := elev.OrderTablePacket{Id: initElev.PhysicalInfo.Id, OrderTable: initElev.OrderTable}
	message := elev.NetworkPacket{OrderTableP: initOTP, PhysicalInfo: initElev.PhysicalInfo}

	address := "255.255.255.255" + ":" + strconv.Itoa(port_hb)

	// Establish udp "connection"
	conn, err := lc.ListenPacket(context.Background(), "udp4", ":0")
	if err != nil {
		log.Printf("TxHeartBeat conn failed! port_hb = %d\n", port_hb)
		log.Printf("TxHeartBeat failed: %v\n", err)
	}
	defer conn.Close()

	dst, err := net.ResolveUDPAddr("udp4", address)
	if err != nil {
		log.Printf("Failed to resolve bcast adress: %v \n", err)
	}

	ticker := time.NewTicker(elev.BCAST_INTERVAL)
	defer ticker.Stop()

	for {
		select {
		case message.OrderTableP.OrderTable = <-ch_updateTxOT:

		case message.PhysicalInfo = <-ch_updateTxPhysicalInfo:

		case <-ticker.C:

			data, err := json.Marshal(message)
			if err != nil {
				log.Println("JSON Marshal error:", err)
				continue
			}

			_, err = conn.WriteTo(data, dst)
			if err != nil {
				log.Println("Error sending message: ", err)
				continue
			}
		}
	}
}

func RxHeartBeat(
	port_hb int,
	ch_RxOrderTableP chan elev.OrderTablePacket,
	ch_RxPhysicalInfo chan elev.ElevatorPhysicalInfo,
	thisElevId int) {

	addr := ":" + strconv.Itoa(port_hb)

	var conn net.PacketConn
	var err error

	// Code ensuring the startup logic is patient.
	// Don't just fail if the port is busy, loop until it's free.
	for {
		conn, err = lc.ListenPacket(context.Background(), "udp4", addr)
		if err == nil {
			log.Printf("Established connection on port %d\n", port_hb)
			break
		}
		log.Printf("Port %d busy, trying again ...\n", port_hb)
		time.Sleep(10 * time.Millisecond)
	}
	defer conn.Close()

	buf := make([]byte, 2048)

	for {

		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			continue
		}

		var recievedInfo elev.NetworkPacket

		err = json.Unmarshal(buf[:n], &recievedInfo)
		if err != nil {
			log.Printf("JSON unmarshal failed: %v\n", err)
			continue
		}

		ch_RxOrderTableP <- recievedInfo.OrderTableP

		if recievedInfo.PhysicalInfo.Id != thisElevId {
			ch_RxPhysicalInfo <- recievedInfo.PhysicalInfo
		}

	}
}

func GetLocalIP() string {
	addr, _ := LocalIP()

	return addr
}
