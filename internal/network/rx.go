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
	conn, err := lc.ListenPacket(context.Background(), "udp4", addr)
	if err != nil {
		log.Printf("[RxOrderTableUDP] ListenPacket failed on port %d: %v\n", port_ot, err)
		return
	}
	defer conn.Close()

	var versionsSeen [elev.N_MAX_ELEVS]uint64
	buf := make([]byte, 4096)

	// Drain all pending control-channel updates before each socket read.
	handleControlUpdates := func() {
		for {
			handledSomething := false

			select {
			case thisRole = <-updateRX_Role:
				log.Println("[RxOrderTableUDP] Role Update")
				handledSomething = true

			case primaryID = <-updateRX_PrimaryID:
				log.Println("[RxOrderTableUDP] PrimaryID Update")
				handledSomething = true

			// Reset versionSeen at resetIndex
			case resetIndex := <-fromRM_ResetVersion:

				// Index Guard
				if resetIndex >= 0 && resetIndex < elev.N_MAX_ELEVS {
					log.Println("[RxOrderTableUDP] Version Reset")
					versionsSeen[resetIndex] = 0
					
				} else {
					log.Fatalf("[RxOrderTableUDP] Ignoring invalid reset index: %d\n", resetIndex)
				}
				handledSomething = true

			default:
			}

			if !handledSomething {
				return
			}
		}
	}

	for {
		// Always service role/primary/reset updates, even if no packets arrive.
		handleControlUpdates()

		// Make ReadFrom wake up periodically so we can go back and check channels.
		if err := conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond)); err != nil {
			log.Printf("[RxOrderTableUDP] SetReadDeadline failed: %v\n", err)
			continue
		}

		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			log.Printf("[RxOrderTableUDP] ReadFrom failed: %v\n", err)
			continue
		}

		var rcvOTP elev.OrderTablePacket
		if err := json.Unmarshal(buf[:n], &rcvOTP); err != nil {
			log.Printf("[RxOrderTableUDP] JSON unmarshal failed: %v\n", err)
			continue
		}

		if rcvOTP.ID < 0 || rcvOTP.ID >= elev.N_MAX_ELEVS {
			log.Fatalf("[RxOrderTableUDP] Ignoring packet with invalid ID: %d\n", rcvOTP.ID)
			continue
		}

		isThisPrimary := thisRole == elev.ER_Primary
		isRcvVersionNewer := rcvOTP.Version > versionsSeen[rcvOTP.ID]
		isRcvVersionInit := rcvOTP.Version <= 1

		isMsgFromSelf := rcvOTP.ID == thisID
		isMsgFromPrimary := rcvOTP.ID == primaryID

		if isThisPrimary {

			if !isMsgFromSelf {

				if isRcvVersionInit || isRcvVersionNewer {
					log.Println("[RxOrderTableUDP] Got message from backup")
					versionsSeen[rcvOTP.ID] = rcvOTP.Version
					fromRX_OTP <- rcvOTP
				}
				//isMsgFromSelf -> Sjekker om vi kan ha samme case uansett om det er fra deg selv eller ikke
			} else {
				if isRcvVersionInit || isRcvVersionNewer {
					log.Println("[RxOrderTableUDP] Got message from backup")
					versionsSeen[rcvOTP.ID] = rcvOTP.Version
					fromRX_OTP <- rcvOTP
				}

			}

			// isThisBackup
		} else {

			if isMsgFromPrimary && isRcvVersionNewer {
				log.Println("[RxOrderTableUDP] Got message from primary")
				versionsSeen[rcvOTP.ID] = rcvOTP.Version
				fromRX_OTP <- rcvOTP
			}
		}
	}
}

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
			log.Fatalf("JSON unmarshal failed: %v\n", err) //Gjorde den fatal
			continue
		}

		select {
		case fromRX_PhysicalInfo <- rcvPhysicalInfo:
		default:
			log.Fatalln("[RxHeartBeat] fromRX_PhysicalInfo full!")
		}

	}
}

func old_RxOrderTable_old(
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

			if isThisPrimary {

				if (isRcvVersionInit || isRcvVersionNewer) && !isMsgFromSelf { //Eskil 12.03 -> reset when a dead elev spawns
					log.Println("[RxOrderTableP] Got message from backup")

					if rcvOTP.ID < 0 || rcvOTP.ID >= elev.N_MAX_ELEVS {
						log.Fatalln("[RxOrderTable] rcvOTP ID out of bounds")
					}

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
