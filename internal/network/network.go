package network

import (
	"net"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Network Config
const BCAST_INTERVAL_HB = 100 * time.Millisecond //HeartBeats
const BCAST_INTERVAL_OT = 100 * time.Millisecond //OrderTablePackets
const BCAST_RCV_IP = "255.255.255.255"

type Ports struct {
	Hardware    int
	OrderTableP int
	HeartBeat   int
}

// UDP Listen Config
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

var localIP_addr string

func localIP() (string, error) {
	if localIP_addr == "" {
		conn, err := net.DialTCP("tcp4", nil, &net.TCPAddr{IP: []byte{8, 8, 8, 8}, Port: 53})
		if err != nil {
			return "", err
		}
		defer conn.Close()
		localIP_addr = strings.Split(conn.LocalAddr().String(), ":")[0]
	}
	return localIP_addr, nil
}

func GetLocalIP() string {
	addr, _ := localIP()
	return addr
}

type VersionTracker struct {
	sync.Mutex
	currentVersion uint64
}

func (v *VersionTracker) increment() uint64 {
	v.Lock()
	defer v.Unlock()
	v.currentVersion++
	return v.currentVersion
}
