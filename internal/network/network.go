package network

import (
	"net"
	"sync"
	"syscall"
	"time"
)

// Network Config
const BCAST_INTERVAL_HB = 30 * time.Millisecond //HeartBeats
const BCAST_INTERVAL_OT = 30 * time.Millisecond //OrderTablePackets
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
