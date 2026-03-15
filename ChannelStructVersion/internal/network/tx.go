package network

import (
	"context"
	"elevator-project-g7/internal/elev"
	"encoding/json"
	"log"
	"net"
	"strconv"
	"time"
)

type TxInputs struct {
	PhysicalInfo     <-chan elev.ElevatorPhysicalInfo
	OrderTablePacket <-chan elev.OrderTablePacket
	Role             <-chan elev.ElevatorRole
}

func TxHeartBeat(
	initElev elev.Elevator,
	portHB int,
	in TxInputs) {

	message := initElev.PhysicalInfo
	address := BCAST_RCV_IP + ":" + strconv.Itoa(portHB)

	// Establish udp "connection"
	conn, err := lc.ListenPacket(context.Background(), "udp4", ":0")
	if err != nil {
		log.Printf("TxHeartBeat conn failed! port_hb = %d\n", portHB)
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

	ticker := time.NewTicker(BCAST_INTERVAL_HB)
	defer ticker.Stop()

	for {
		select {

		case newPhysicalInfo := <-in.PhysicalInfo:
			message = newPhysicalInfo
			broadcast()
			ticker.Reset(BCAST_INTERVAL_HB)

		case <-ticker.C:
			broadcast()
			ticker.Reset(BCAST_INTERVAL_HB)
		}
	}
}

func TxOrderTable(
	initElev elev.Elevator,
	portOT int,
	in TxInputs,
) {

	address := BCAST_RCV_IP + ":" + strconv.Itoa(portOT)
	conn, _ := lc.ListenPacket(context.Background(), "udp4", ":0")
	defer conn.Close()
	dst, _ := net.ResolveUDPAddr("udp4", address)

	var latestPacket elev.OrderTablePacket
	versionTracker := &VersionTracker{}

	ticker := time.NewTicker(BCAST_INTERVAL_OT)

	for {
		select {

		case newOTP := <-in.OrderTablePacket:
			// Increment version before updating packet info
			var nextVersion uint64
			nextVersion = versionTracker.increment()

			newOTP.Version = nextVersion
			latestPacket = newOTP

		case <-ticker.C:
			data, _ := json.Marshal(latestPacket)
			conn.WriteTo(data, dst)

		}
	}
}
