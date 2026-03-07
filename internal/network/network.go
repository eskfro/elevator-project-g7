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

	ticker := time.NewTicker(elev.BCAST_INTERVAL)
	defer ticker.Stop()

	for {
		select {

		case newPhysicalInfo := <-ch_updateTX_PhysicalInfo:
			message = newPhysicalInfo
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

		var recievedInfo elev.ElevatorPhysicalInfo

		err = json.Unmarshal(buf[:n], &recievedInfo)
		if err != nil {
			log.Printf("JSON unmarshal failed: %v\n", err)
			continue
		}

		// Send ElevatorPhysicalInfo heartbeat if not self
		// if recievedInfo.Id != thisElevId {
		// 	select {
		// 	case ch_fromRX_PhysicalInfo <- recievedInfo:

		// 	default:
		// 		log.Println("fromRX_PhysicalInfo is full!")

		// 	}
		// }
		ch_fromRX_PhysicalInfo <- recievedInfo
	}
}

func GetLocalIP() string {
	addr, _ := LocalIP()
	return addr
}

func TxOrderTableTCP(elevId int, port_ot int, ch_updateTX_OTP <-chan elev.OrderTablePacket) {

	targetPort := port_ot + elevId
	addr := ":" + strconv.Itoa(targetPort)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("TCP Listen error: %v", err)
	}
	defer listener.Close()

	log.Printf("[TX TCP] Waiting for Backup on %s\n", addr)

	for {
		log.Println("[TX TCP] Loop Warning")
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}

		log.Printf("[TX TCP] Backup connected: %s", conn.RemoteAddr())

		encoder := json.NewEncoder(conn)

		for otp := range ch_updateTX_OTP {
			log.Println("[TX TCP] OrderTable Packet update")
			err := encoder.Encode(otp)
			if err != nil {
				log.Printf("[TX TCP] Backup disconnected: %v", err)
				conn.Close()
				break
			}
		}
	}
}

func RxOrderTableTCP(
	initialIp string,
	port_ot int,
	ch_updateRX_PrimaryIp <-chan string,
	ch_fromRX_OTP chan<- elev.OrderTablePacket,
	initPrimaryId int,
	ch_updateRX_PrimaryId <-chan int,
) {

	primaryIp := elev.INVALID_PRIMARY_IP
	primaryId := initPrimaryId

	for {
		log.Println("[RX TCP] Loop Warning (1)")

		if primaryIp == elev.INVALID_PRIMARY_IP {
			log.Println("[RX TCP] Waiting for valid IP...")
			select {
			case primaryIp = <-ch_updateRX_PrimaryIp:
			case primaryId = <-ch_updateRX_PrimaryId:
			}
		}

		// Calculate the primary port
		targetPort := port_ot + primaryId
		addr := primaryIp + ":" + strconv.Itoa(targetPort)
		log.Printf("[RX TCP] Dialing %s", addr)

		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err != nil {
			log.Println("[RX TCP] Idk man")
			select {
			case primaryIp = <-ch_updateRX_PrimaryIp:
				log.Printf("[RX TCP] New IP: %s", primaryIp)
			case primaryId = <-ch_updateRX_PrimaryId:
				log.Printf("[RX TCP] New PrimaryId: %d", primaryId)
			case <-time.After(time.Second):
			}
			continue
		}

		log.Printf("[RX TCP] Connected to %s", addr)

		decoder := json.NewDecoder(conn)

		for {
			// Decode message
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			var otp elev.OrderTablePacket
			err := decoder.Decode(&otp)

			//Handle error decoding message (ugly)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					select {
					case primaryIp = <-ch_updateRX_PrimaryIp:
						log.Println("[RX TCP] IP changed, reconnecting")
						conn.Close()
						goto reconnect

					case primaryId = <-ch_updateRX_PrimaryId:
						log.Println("[RX TCP] PrimaryID changed, reconnecting")
						conn.Close()
						goto reconnect

					default:
						continue
					}
				}
				log.Printf("[RX TCP] Connection lost: %v", err)
				conn.Close()
				break
			}

			// Send OrderTablePacket to
			log.Println("[RX TCP] Sending OrderTable Package")
			ch_fromRX_OTP <- otp
		}

	reconnect:
	}
}

func RxOrderTableTCP2(
	initialIP string,
	port_ot int,
	ch_updateIP <-chan string,
	ch_fromRX_OTP chan<- elev.OrderTablePacket,
	initPrimaryId int,
	ch_updatePrimaryId <-chan int, // Receiver channel
) {
	currentIP := initialIP
	primaryId := initPrimaryId

	for {
		// 1. Guard against invalid state
		if currentIP == elev.INVALID_PRIMARY_IP {
			log.Println("[RX TCP] Waiting for valid IP...")
			currentIP = <-ch_updateIP
			continue
		}

		// 2. RE-CALCULATE port here so it updates when primaryId changes
		targetPort := port_ot + primaryId
		addr := currentIP + ":" + strconv.Itoa(targetPort)

		log.Printf("[RX TCP] Attempting dial: %s", addr)
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)

		if err != nil {
			// Check for updates during retry backoff
			select {
			case currentIP = <-ch_updateIP:
				log.Printf("[RX TCP] New Ip: %s\n", currentIP)

			case primaryId = <-ch_updatePrimaryId:
				log.Printf("[RX TCP] New PrimaryId: %d\n", primaryId)

			case <-time.After(1 * time.Second):
			}
			continue
		}

		// 3. Connection Successful
		log.Printf("[RX TCP] Connected to %s", addr)
		decoder := json.NewDecoder(conn)

		for {
			// OPTIONAL: Use SetReadDeadline here if you want to
			// periodically check for IP/ID updates while connected.

			var otp elev.OrderTablePacket
			err := decoder.Decode(&otp)
			if err != nil {
				log.Printf("[RX TCP] Connection broken: %v", err)
				break
			}
			ch_fromRX_OTP <- otp
		}
		conn.Close()
	}
}

func TxOrderTableTCP2(port_ot int, ch_updateTX_OTP <-chan elev.OrderTablePacket) {
	addr := ":" + strconv.Itoa(port_ot)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("TCP Listen error: %v", err)
	}
	defer listener.Close()

	log.Printf("[TX TCP] Primary: Waiting for Backup to connect on TCP %d...\n", port_ot)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		log.Printf("Backup connected: %s", conn.RemoteAddr())

		// Handle the connection in a helper function
		go func(c net.Conn) {
			defer c.Close()
			encoder := json.NewEncoder(c)

			for {
				// Wait for an actual update from the channel
				otp, ok := <-ch_updateTX_OTP
				if !ok {
					return
				}

				err := encoder.Encode(otp)
				if err != nil {
					log.Printf("[TX TCP] Send failed (Backup disconnected): %v", err)
					return
				}
				log.Println("Sent OrderTable update via TCP")
			}
		}(conn)
	}
}
