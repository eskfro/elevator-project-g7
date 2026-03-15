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

type RxInputs struct {
	Role         <-chan elev.ElevatorRole
	PrimaryID    <-chan int
	ResetVersion <-chan int
}

type RxOutputs struct {
	PhysicalInfo     chan<- elev.ElevatorPhysicalInfo
	OrderTablePacket chan<- elev.OrderTablePacket
}

func RxHeartBeat(
	portHB int,
	thisElevId int,
	out RxOutputs,
) {

	addr := ":" + strconv.Itoa(portHB)

	var conn net.PacketConn
	var err error

	// Code ensuring the startup logic is patient.
	// Don't just fail if the port is busy, loop until it's free.
	for {
		conn, err = lc.ListenPacket(context.Background(), "udp4", addr)
		if err == nil {
			log.Printf("[UDP] Established connection on port %d\n", portHB)
			break
		}
		log.Printf("Port %d busy, trying again ...\n", portHB)
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
		case out.PhysicalInfo <- rcvPhysicalInfo:
		default:
			log.Println("[RxHeartBeat] fromRX_PhysicalInfo full!")
		}

	}
}

func RxOrderTable(
	initElev elev.Elevator,
	portOT int,
	in RxInputs,
	out RxOutputs,
) {

	thisRole := initElev.PhysicalInfo.Role
	thisId := initElev.PhysicalInfo.Id
	primaryId := initElev.PhysicalInfo.PrimaryId

	addr := ":" + strconv.Itoa(portOT)
	conn, _ := lc.ListenPacket(context.Background(), "udp4", addr)

	var versionsSeen [elev.N_MAX_ELEVS]uint64

	buf := make([]byte, 4096)

	for {
		select {

		case thisRole = <-in.Role:
			log.Println("[RxOrderTableUDP] Role Update")

		case primaryId = <-in.PrimaryID:
			log.Println("[RxOrderTableUDP] PrimaryId Update")

		case resetIndex := <-in.ResetVersion: //TODO: Må resetVersion vere uint64 også?
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
			isRcvVersionNewer := rcvOTP.Version > versionsSeen[rcvOTP.Id]
			isRcvVersionInit := rcvOTP.Version <= 1

			isMsgFromSelf := rcvOTP.Id == thisId
			isMsgFromPrimary := rcvOTP.Id == primaryId

			if isThisPrimary && !isMsgFromSelf {

				if isRcvVersionInit || isRcvVersionNewer { //Eskil 12.03 -> reset when a dead elev spawns
					log.Println("[RxOrderTableP] Got message from backup")
					versionsSeen[rcvOTP.Id] = rcvOTP.Version
					out.OrderTablePacket <- rcvOTP
					continue
				}

				// isThisBackup //TODO: Denne kommentaren stemmer vel ikkje heilt?
			} else {

				if isMsgFromPrimary && isRcvVersionNewer {
					log.Println("[RxOrderTableP] Got message from primary")
					versionsSeen[rcvOTP.Id] = rcvOTP.Version
					out.OrderTablePacket <- rcvOTP
					continue
				}
			}
		}
	}
}
