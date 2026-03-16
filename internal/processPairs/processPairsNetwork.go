package processpairs

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

const LOOPBACK_IP = "127.0.0.1"

func localHeartbeatPort(id int) int { return 51000 + id }
func localSnapshotPort(id int) int  { return 52000 + id }

func txHeartbeat(port int) {
	lc := listenConfig()
	conn, err := lc.ListenPacket(context.Background(), "udp4", ":0")
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	destination, err := net.ResolveUDPAddr("udp4", LOOPBACK_IP+":"+strconv.Itoa(port))
	if err != nil {
		log.Fatalln(err)
	}

	ticker := time.NewTicker(PP_HEARTBEAT_INTERVAL)
	defer ticker.Stop()

	for range ticker.C {
		_, _ = conn.WriteTo([]byte("hb"), destination)
	}
}

func rxHeartbeat(port int, out chan<- struct{}, done <-chan struct{}) {
	lc := listenConfig()
	conn, err := lc.ListenPacket(context.Background(), "udp4", ":"+strconv.Itoa(port))
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	buf := make([]byte, 16)
	for {
		select {
		case <-done:
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		_, _, err := conn.ReadFrom(buf)
		if err != nil {
			continue
		}
		select {
		case out <- struct{}{}:
		default:
		}
	}
}

func txSnapshots(port int, in <-chan elev.Elevator) {
	lc := listenConfig()
	conn, err := lc.ListenPacket(context.Background(), "udp4", ":0")
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	destination, err := net.ResolveUDPAddr("udp4", LOOPBACK_IP+":"+strconv.Itoa(port))
	if err != nil {
		log.Fatalln(err)
	}

	var version uint64

	for snapshot := range in {
		version++
		data, err := json.Marshal(Snapshot{Elevator: snapshot, Version: version})
		if err != nil {
			continue
		}
		_, _ = conn.WriteTo(data, destination)
	}
}

func rxSnapshots(port int, out chan<- Snapshot, done <-chan struct{}) {
	lc := listenConfig()
	conn, err := lc.ListenPacket(context.Background(), "udp4", ":"+strconv.Itoa(port))
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	buf := make([]byte, 65535)
	for {
		select {
		case <-done:
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			continue
		}
		var snap Snapshot
		if err := json.Unmarshal(buf[:n], &snap); err != nil {
			continue
		}

		select {
		case out <- snap:
		default:
		}
	}
}

func listenConfig() net.ListenConfig {
	return net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			})
		},
	}
}
