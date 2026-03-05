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
	ch_updateTX_OTP chan elev.OrderTablePacket,
	ch_updateTX_PacketId chan int,
	ch_updateTX_PhysicalInfo chan elev.ElevatorPhysicalInfo) {

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

	broadcast := func() {
		data, err := json.Marshal(message)
		if err != nil {
			log.Println("JSON Marshal error:", err)
		}
		_, err = conn.WriteTo(data, dst)
		if err != nil {
			log.Println("Error sending message: ", err)
		}
	}

	ticker := time.NewTicker(elev.BCAST_INTERVAL)
	defer ticker.Stop()

	for {
		select {
		case newOrderTableP := <-ch_updateTX_OTP:
			message.OrderTableP = newOrderTableP
			broadcast()
			ticker.Reset(elev.BCAST_INTERVAL)

		case newPacketId := <-ch_updateTX_PacketId:
			message.OrderTableP.Id = newPacketId
			broadcast()
			ticker.Reset(elev.BCAST_INTERVAL)

		case newPhysicalInfo := <-ch_updateTX_PhysicalInfo:
			message.PhysicalInfo = newPhysicalInfo
			broadcast()
			ticker.Reset(elev.BCAST_INTERVAL)

		case <-ticker.C:
			broadcast()
			ticker.Reset(elev.BCAST_INTERVAL)
		}
	}
}

func RxHeartBeat(
	port_hb int,
	ch_fromRX_OrderTableP chan elev.OrderTablePacket,
	ch_fromRX_PhysicalInfo chan elev.ElevatorPhysicalInfo,
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

		select {
		case ch_fromRX_OrderTableP <- recievedInfo.OrderTableP:

		default:
			log.Println("OrderTableP case in OrderControl is full!")
		}
		// Send OrderTablePacket to OrderControl

		// Send ElevatorPhysicalInfo heartbeat if not self
		if recievedInfo.PhysicalInfo.Id != thisElevId {
			select {
			case ch_fromRX_PhysicalInfo <- recievedInfo.PhysicalInfo:

			default:
				log.Println("fromRX_PhysicalInfo is full!")

			}
		}
	}
}

func GetLocalIP() string {
	addr, _ := LocalIP()
	return addr
}
