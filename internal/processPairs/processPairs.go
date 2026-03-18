package processpairs

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	eventloop "elevator-project-g7/internal/eventLoop"
	"elevator-project-g7/internal/network"
	"elevator-project-g7/internal/timer"
	"log"
	"os/exec"
	"strconv"
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
	ID int,
	ports network.Ports,
	ppRole string,
) {
	switch Role(ppRole) {
	case PP_MASTER:
		log.Println("[PP START] ppRole = Master")
		startMaster(ID, ports)

	case PP_SLAVE:
		log.Println("[PP START] ppRole = Slave")
		startSlave(ID, ports)

	default:
		log.Fatalln("[PP START] ppRole not 0 or 1")
	}
}

func startMaster(
	ID int,
	ports network.Ports,
) {
	snapshotTx := make(chan elev.Elevator, 32)

	elevio.InitPhysicalElevator("localhost", ports.Hardware, elev.N_FLOORS)
	elevator := elev.CreateElevator(ID, ports.Hardware, network.GetLocalIP())

	fromIO_BtnPress, fromIO_Floor, fromIO_Obstruction := elevio.CreateHardwareChannels()

	go txHeartbeat(localHeartbeatPort(ID))
	go txSnapshots(localSnapshotPort(ID), snapshotTx)

	spawnSlave(ID, ports)

	snapshotTx <- elevator

	eventloop.Start(elevator, ports, fromIO_BtnPress, fromIO_Floor, fromIO_Obstruction, snapshotTx)
}

func startSlave(
	ID int,
	ports network.Ports,
) {
	hbRx := make(chan struct{}, 32)
	snapshotRx := make(chan Snapshot, 32)
	done := make(chan struct{})

	timeout := timer.New(PP_HEARTBEAT_TIMEOUT)
	defer timeout.Close()

	go rxHeartbeat(localHeartbeatPort(ID), hbRx, done)
	go rxSnapshots(localSnapshotPort(ID), snapshotRx, done)

	timeout.Start()

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
			log.Println("[PP] Master timed out, taking over")

			if !haveSnapshot {
				mirrorElev = elev.CreateElevator(ID, ports.Hardware, network.GetLocalIP())

			}
			//Slave takes control over hardware
			snapshotTx := make(chan elev.Elevator, 32)

			elevio.InitPhysicalElevator("localhost", ports.Hardware, elev.N_FLOORS)

			fromIO_BtnPress, fromIO_Floor, fromIO_Obstruction := elevio.CreateHardwareChannels()

			go txHeartbeat(localHeartbeatPort(ID))
			go txSnapshots(localSnapshotPort(ID), snapshotTx)

			mirrorElev.PhysicalInfo.Role = elev.ER_Backup
			mirrorElev.PhysicalInfo.PrimaryID = elev.INVALID_PRIMARY_ID
			mirrorElev.PhysicalInfo.MotorDir = elevio.MD_Stop

			if mirrorElev.PhysicalInfo.Movement == elev.EM_DoorOpen {
				elevio.SetDoorOpenLamp(true)
			}

			if elevio.GetObstruction() {
				mirrorElev.PhysicalInfo.Obstructed = true
			}

			close(done)
			spawnSlave(ID, ports)
			snapshotTx <- mirrorElev
			eventloop.Start(mirrorElev, ports, fromIO_BtnPress, fromIO_Floor, fromIO_Obstruction, snapshotTx)
			return
		}
	}
}

func spawnSlave(ID int, ports network.Ports) {

	strID := strconv.Itoa(ID)
	title := "Elevator " + strID

	cmd := exec.Command(
		"gnome-terminal",
		"--title="+title,
		"--",
		"bash",
		"-c",
		"./"+TARGET+" "+
			strconv.Itoa(ID)+" "+
			strconv.Itoa(ports.Hardware)+" "+
			strconv.Itoa(ports.HeartBeat)+" "+
			strconv.Itoa(ports.OrderTableP)+" "+
			string(PP_SLAVE),
	)

	err := cmd.Start()
	if err != nil {
		log.Println("Spawn error:", err)
	}
}
