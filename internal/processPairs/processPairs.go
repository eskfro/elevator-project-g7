package processpairs

import (
	"context"
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	eventloop "elevator-project-g7/internal/eventLoop"
	"elevator-project-g7/internal/network"
	"elevator-project-g7/internal/timer"
	"encoding/json"
	"log"
	"net"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

type Role string

const (
	PP_MASTER Role = "1"
	PP_SLAVE  Role = "0"
)

type Snapshot struct {
	Version  uint64
	Elevator elev.Elevator
}

const TARGET = "out"

const PP_HEARTBEAT_INTERVAL = 200 * time.Millisecond
const PP_HEARTBEAT_TIMEOUT = 1 * time.Second

func Start(
	id int,
	ports network.Ports,
	ppRole string,
) {
	switch Role(ppRole) {
	case PP_MASTER:
		log.Println("[PP START] ppRole = Master")
		startMaster(id, ports)

	case PP_SLAVE:
		log.Println("[PP START] ppRole = Slave")
		startSlave(id, ports)

	default:
		log.Fatalln("[PP START] ppRole not 0 or 1")
	}
}

func startMaster(
	id int,
	ports network.Ports,
) {
	// Start Process Pair Slave
	
	elevio.InitPhysicalElevator("localhost", ports.Hardware, elev.N_FLOORS)
	elevator := elev.CreateElevator(id, ports.Hardware, network.GetLocalIP())
	
	fromIO_BtnPress, fromIO_Floor, fromIO_Obstruction := elevio.Inputs()
	
	snapshotTx := make(chan elev.Elevator, 32)
	go txHeartbeat(localHeartbeatPort(id))
	go txSnapshots(localSnapshotPort(id), snapshotTx)
	spawnSlave(id, ports)
	
	snapshotTx <- elevator

	eventloop.Start(elevator, ports, fromIO_BtnPress, fromIO_Floor, fromIO_Obstruction, snapshotTx)
}

func startSlave(
	id int,
	ports network.Ports,
) {
	hbRx := make(chan struct{}, 32)
	snapshotRx := make(chan Snapshot, 32)

	done := make(chan struct{})
	go rxHeartbeat(localHeartbeatPort(id), hbRx, done)
	go rxSnapshots(localSnapshotPort(id), snapshotRx, done)

	timeout := timer.New(PP_HEARTBEAT_TIMEOUT)
	timeout.Start()
	defer timeout.Close()

	var mirrorElev elev.Elevator
	var lastVersion uint64
	haveSnapshot := false

	for {
		select {
		case <-hbRx:
			timeout.Start()

		case snapshot := <-snapshotRx:
			if snapshot.Version > lastVersion {
				lastVersion = snapshot.Version
				mirrorElev = snapshot.Elevator
				haveSnapshot = true
			}

		case <-timeout.C:
			log.Println("[PP] master timed out, taking over")

			if !haveSnapshot {
				mirrorElev = elev.CreateElevator(id, ports.Hardware, network.GetLocalIP())
			}
			//Slave takes control over hardware
			elevio.InitPhysicalElevator("localhost", ports.Hardware, elev.N_FLOORS)
			fromIO_BtnPress, fromIO_Floor, fromIO_Obstruction := elevio.Inputs()

			snapshotTx := make(chan elev.Elevator, 32)
			go txHeartbeat(localHeartbeatPort(id))
			go txSnapshots(localSnapshotPort(id), snapshotTx)

			// Check these if troubleshooting
			mirrorElev.PhysicalInfo.Role = elev.ER_Backup
			mirrorElev.PhysicalInfo.PrimaryId = elev.INVALID_PRIMARY_ID
			mirrorElev.PhysicalInfo.Movement = elev.EM_Idle
			mirrorElev.PhysicalInfo.MotorDir = elevio.MD_Stop

			snapshotTx <- mirrorElev

			close(done)
			spawnSlave(id, ports)
			eventloop.Start(mirrorElev, ports, fromIO_BtnPress, fromIO_Floor, fromIO_Obstruction, snapshotTx)
			return
		}
	}
}

func spawnSlave2(id int, ports network.Ports) {
	log.Println("[PROCESS PAIRS]============================ I HAVE TRIED TO SPAWN A BACKUP")
	cmd := exec.Command(
		"gnome-terminal",
		"--",
		"./"+TARGET,
		strconv.Itoa(id),
		strconv.Itoa(ports.Hardware),
		strconv.Itoa(ports.HeartBeat),
		strconv.Itoa(ports.OrderTableP),
		string(PP_SLAVE)+"; read",
	)
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
}

func spawnSlave(id int, ports network.Ports) {
	log.Println("[PROCESS PAIRS]============================ I HAVE TRIED TO SPAWN A BACKUP")

	cmd := exec.Command(
		"gnome-terminal",
		"--",
		"bash",
		"-c",
		"./"+TARGET+" "+
			strconv.Itoa(id)+" "+
			strconv.Itoa(ports.Hardware)+" "+
			strconv.Itoa(ports.HeartBeat)+" "+
			strconv.Itoa(ports.OrderTableP)+" "+
			string(PP_SLAVE)+"; read",
	)

	err := cmd.Start()
	if err != nil {
		log.Println("Spawn error:", err)
	}
}

func localHeartbeatPort(id int) int { return 51000 + id }
func localSnapshotPort(id int) int  { return 52000 + id }

func txHeartbeat(port int) {
	lc := listenConfig()
	conn, err := lc.ListenPacket(context.Background(), "udp4", ":0")
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	destination, err := net.ResolveUDPAddr("udp4", "127.0.0.1:"+strconv.Itoa(port))
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

	destination, err := net.ResolveUDPAddr("udp4", "127.0.0.1:"+strconv.Itoa(port))
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
