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
			log.Println("[RX TCP] Sending Rcvd OrderTable Package in Channel")

			ch_fromRX_OTP <- otp
		}

	reconnect:
	}
}
