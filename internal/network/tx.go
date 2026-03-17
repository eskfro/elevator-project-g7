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

func TxHeartBeat(
	initElev elev.Elevator,
	port_hb int,
	updateTX_PhysicalInfo <-chan elev.ElevatorPhysicalInfo) {

	message := initElev.PhysicalInfo
	address := BCAST_RCV_IP + ":" + strconv.Itoa(port_hb)

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
			// log.Println("Error sending message: ", err)
		}
	}

	ticker := time.NewTicker(BCAST_INTERVAL_HB)
	defer ticker.Stop()

	for {
		select {

		case newPhysicalInfo := <-updateTX_PhysicalInfo:
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
	port_ot int,
	updateTX_OTP <-chan elev.OrderTablePacket,
) {

	address := BCAST_RCV_IP + ":" + strconv.Itoa(port_ot)
	conn, _ := lc.ListenPacket(context.Background(), "udp4", ":0")
	defer conn.Close()
	dst, _ := net.ResolveUDPAddr("udp4", address)

	var latestPacket elev.OrderTablePacket
	latestPacket.ID = elev.INVALID_ELEVATOR_ID
	versionTracker := &VersionTracker{}

	ticker := time.NewTicker(BCAST_INTERVAL_OT)

	for {
		select {

		// Update info to transmitt
		case newOTP := <-updateTX_OTP:
			// Increment version before updating packet info
			var nextVersion uint64
			nextVersion = versionTracker.increment()

			newOTP.Version = nextVersion
			latestPacket = newOTP

		// Ticker for tx interval
		case <-ticker.C:

			// Guard so we dont send init OTP
			if latestPacket.ID != elev.INVALID_ELEVATOR_ID {
				data, _ := json.Marshal(latestPacket)
				conn.WriteTo(data, dst)
			}

		}
	}
}
