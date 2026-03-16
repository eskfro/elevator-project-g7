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

func RxHeartBeat(
	port_hb int,
	fromRX_PhysicalInfo chan<- elev.ElevatorPhysicalInfo,
	thisElevID int) {

	addr := ":" + strconv.Itoa(port_hb)

	var conn net.PacketConn
	var err error

	// Code ensuring the startup logic is patient.
	// Don't just fail if the port is busy, loop until it's free.
	for {
		conn, err = lc.ListenPacket(context.Background(), "udp4", addr)
		if err == nil {
			log.Printf("[UDP] Established connection on port %d\n", port_hb)
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

		var rcvPhysicalInfo elev.ElevatorPhysicalInfo

		err = json.Unmarshal(buf[:n], &rcvPhysicalInfo)
		if err != nil {
			log.Printf("JSON unmarshal failed: %v\n", err)
			continue
		}

		select {
		case fromRX_PhysicalInfo <- rcvPhysicalInfo:
		default:
			log.Println("[RxHeartBeat] fromRX_PhysicalInfo full!")
		}

	}
}

func RxOrderTable(
	initElev elev.Elevator,
	port_ot int,
	updateRX_Role <-chan elev.ElevatorRole,
	updateRX_PrimaryID <-chan int,
	fromRM_ResetVersion <-chan int,
	fromRX_OTP chan<- elev.OrderTablePacket,
) {

	thisRole := initElev.PhysicalInfo.Role
	thisID := initElev.PhysicalInfo.ID
	primaryID := initElev.PhysicalInfo.PrimaryID

	addr := ":" + strconv.Itoa(port_ot)
	conn, _ := lc.ListenPacket(context.Background(), "udp4", addr)

	var versionsSeen [elev.N_MAX_ELEVS]uint64

	buf := make([]byte, 4096) // Larger buffer for the whole table

	for {
		select {

		case thisRole = <-updateRX_Role:
			log.Println("[RxOrderTableUDP] Role Update")

		case primaryID = <-updateRX_PrimaryID:
			log.Println("[RxOrderTableUDP] PrimaryId Update")

		case resetIndex := <-fromRM_ResetVersion: //Eskil 12.03
			log.Println("[RxOrderTableUDP] Version Reset")
			versionsSeen[resetIndex] = 0

		default:
			n, _, err := conn.ReadFrom(buf)
			if err != nil {
				continue
			}

			var rcvOTP elev.OrderTablePacket
			json.Unmarshal(buf[:n], &rcvOTP)

			isThisPrimary := thisRole == elev.ER_Primary
			isRcvVersionNewer := rcvOTP.Version > versionsSeen[rcvOTP.ID]
			isRcvVersionInit := rcvOTP.Version <= 1

			isMsgFromSelf := rcvOTP.ID == thisID
			isMsgFromPrimary := rcvOTP.ID == primaryID

			if isThisPrimary && !isMsgFromSelf {

				if isRcvVersionInit || isRcvVersionNewer { //Eskil 12.03 -> reset when a dead elev spawns
					log.Println("[RxOrderTableP] Got message from backup")
					versionsSeen[rcvOTP.ID] = rcvOTP.Version
					fromRX_OTP <- rcvOTP
					continue
				}

				// isThisBackup //TODO: Denne kommentaren stemmer vel ikkje heilt?
			} else {

				if isMsgFromPrimary && isRcvVersionNewer {
					log.Println("[RxOrderTableP] Got message from primary")
					versionsSeen[rcvOTP.ID] = rcvOTP.Version
					fromRX_OTP <- rcvOTP
					continue
				}
			}
		}
	}
}
