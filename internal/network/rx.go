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

		isFromSelf := rcvPhysicalInfo.Id == thisElevId

		if !isFromSelf {
			select {
			case ch_fromRX_PhysicalInfo <- rcvPhysicalInfo:
			default:
				log.Println("[RxHeartBeat] fromRX_PhysicalInfo full!")
			}
			// Ja dette er bra kodekvalitet.
		} else {
			select {
			case ch_fromRX_PhysicalInfo <- rcvPhysicalInfo:
			default:
				log.Println("[RxHeartBeat] fromRX_PhysicalInfo full!")
			}
		}

	}
}

func RxOrderTableUDP(
	initElev elev.Elevator,
	port_ot int,
	ch_fromRX_OTP chan<- elev.OrderTablePacket,
	updateRX_Role chan elev.ElevatorRole,
	updateRX_PrimaryId chan int,
) {

	thisRole := initElev.PhysicalInfo.Role
	thisId := initElev.PhysicalInfo.Id
	primaryId := initElev.PhysicalInfo.PrimaryId

	addr := ":" + strconv.Itoa(port_ot)
	conn, _ := lc.ListenPacket(context.Background(), "udp4", addr)

	var versionsSeen [elev.N_MAX_ELEVS]uint64

	buf := make([]byte, 4096) // Larger buffer for the whole table

	for {
		select {

		case thisRole = <-updateRX_Role:

		case primaryId = <-updateRX_PrimaryId:

		default:
			n, _, err := conn.ReadFrom(buf)
			if err != nil {
				continue
			}

			var rcvOTP elev.OrderTablePacket
			json.Unmarshal(buf[:n], &rcvOTP)

			isThisPrimary := thisRole == elev.ER_Primary
			isRcvVersionNewer := rcvOTP.Version > versionsSeen[rcvOTP.Id]
			isRcvVersionOne := rcvOTP.Version == 1

			isMsgFromSelf := rcvOTP.Id == thisId
			isMsgFromPrimary := rcvOTP.Id == primaryId

			if isThisPrimary && !isMsgFromSelf {

				if isRcvVersionOne || isRcvVersionNewer { //Eskil 12.03 -> reset when a dead elev spawns
					versionsSeen[rcvOTP.Id] = rcvOTP.Version
					ch_fromRX_OTP <- rcvOTP
					continue

				}

				// isThisBackup
			} else {

				if isMsgFromPrimary && isRcvVersionNewer {
					log.Println("[RxOrderTableP] Got message from primary")
					versionsSeen[rcvOTP.Id] = rcvOTP.Version
					ch_fromRX_OTP <- rcvOTP
					continue
				}
			}
		}
	}
}
