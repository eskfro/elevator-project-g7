package network

import (
	"context"
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/ordercontrol"
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
	ch_updateTX_PhysicalInfo chan elev.ElevatorPhysicalInfo) {

	message := initElev.PhysicalInfo

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

	ticker := time.NewTicker(elev.BCAST_INTERVAL_HB)
	defer ticker.Stop()

	for {
		select {

		case newPhysicalInfo := <-ch_updateTX_PhysicalInfo:
			message = newPhysicalInfo
			broadcast()
			ticker.Reset(elev.BCAST_INTERVAL_HB)

		case <-ticker.C:
			broadcast()
			ticker.Reset(elev.BCAST_INTERVAL_HB)
		}
	}
}

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

		isNotFromSelf := rcvPhysicalInfo.Id != thisElevId

		if isNotFromSelf {
			select {
			case ch_fromRX_PhysicalInfo <- rcvPhysicalInfo:
			default:
				log.Println("[RxHeartBeat] fromRX_PhysicalInfo full!")
			}
		}

	}
}

func GetLocalIP() string {
	addr, _ := LocalIP()
	return addr
}

func TxOrderTableUDP(
	initElev elev.Elevator,
	port_ot int,
	ch_updateTX_OTP <-chan elev.OrderTablePacket,
	updateTX_Role chan elev.ElevatorRole,
) {

	thisRole := initElev.PhysicalInfo.Role

	// 1. Setup Connection (Reusing your 'lc' logic)
	address := "255.255.255.255:" + strconv.Itoa(port_ot)
	conn, _ := lc.ListenPacket(context.Background(), "udp4", ":0")
	defer conn.Close()
	dst, _ := net.ResolveUDPAddr("udp4", address)

	// Store the latest packet to rebroadcast periodically
	var latestPacket elev.OrderTablePacket
	versionTracker := &ordercontrol.VersionTracker{}

	ticker := time.NewTicker(elev.BCAST_INTERVAL_OT * time.Millisecond) // Periodic "Gospel" broadcast

	for {
		select {

		case thisRole = <-updateTX_Role:

		case newOTP := <-ch_updateTX_OTP:
			// Increment version before sending
			var nextVersion uint64
			isThisPrimary := thisRole == elev.ER_Primary

			if isThisPrimary {
				nextVersion = versionTracker.Increment()
			} else {
				nextVersion = versionTracker.Get()
			}

			newOTP.Version = nextVersion

			latestPacket = newOTP

			// Send immediately
			data, _ := json.Marshal(latestPacket)
			conn.WriteTo(data, dst)
			log.Printf("[TxOrderTableUDP] OTP sent | vNum = %d\n", nextVersion)

		case <-ticker.C:
			// Periodic rebroadcast of the last known state
			// This helps elevators that were temporarily offline catch up
			if latestPacket.Version > 0 {
				data, _ := json.Marshal(latestPacket)
				conn.WriteTo(data, dst)
			}
		}
	}
}

func RxOrderTableUDP(
	initElev elev.Elevator,
	port_ot int,
	ch_fromRX_OTP chan<- elev.OrderTablePacket,
	updateRX_Role chan elev.ElevatorRole, // TODO: Make this channel update thisRole
) {

	thisRole := initElev.PhysicalInfo.Role
	thisId := initElev.PhysicalInfo.Id

	addr := ":" + strconv.Itoa(port_ot)
	conn, _ := lc.ListenPacket(context.Background(), "udp4", addr)

	// Map to track the highest version seen from each Elevator ID
	// key: ElevatorId, value: MaxVersion
	var versionsSeen [elev.N_MAX_ELEVS]uint64

	buf := make([]byte, 4096) // Larger buffer for the whole table

	for {
		select {

		case thisRole = <-updateRX_Role:

		default:
			n, _, err := conn.ReadFrom(buf)
			if err != nil {
				continue
			}

			var rcvOTP elev.OrderTablePacket
			json.Unmarshal(buf[:n], &rcvOTP)

			isThisPrimary := thisRole == elev.ER_Primary
			isRcvVersionNewer := rcvOTP.Version > versionsSeen[rcvOTP.Id]
			//isRcvVersionEqual := rcvOTP.Version == versionsSeen[rcvOTP.Id]
			isMsgNotFromSelf := rcvOTP.Id != thisId

			if isThisPrimary {
				if isMsgNotFromSelf { //&& (isRcvVersionNewer || isRcvVersionEqual)
					versionsSeen[rcvOTP.Id] = rcvOTP.Version
					ch_fromRX_OTP <- rcvOTP
					continue
				}
				// isThisBackup
			} else {
				if isRcvVersionNewer {
					versionsSeen[rcvOTP.Id] = rcvOTP.Version
					ch_fromRX_OTP <- rcvOTP
					continue
				}
			}
		}
	}
}
