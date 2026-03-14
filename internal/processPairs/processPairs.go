package processpairs

import (
	"elevator-project-g7/internal/network"
	"log"
	"os/exec"
	"strconv"
)

const roleMaster = "1"
const roleSlave = "0"
const TARGET = "out"

func Start(ID int, ports network.Ports, ppRole string) {
	switch ppRole {
	case roleMaster:
		log.Println("[PP START] ppRole = Master")

		// Start Process Pair Slave
		cmd := exec.Command(
			"gnome-terminal",
			"--",
			"./"+TARGET,
			strconv.Itoa(ID),
			strconv.Itoa(ports.Hardware),
			strconv.Itoa(ports.HeartBeat),
			strconv.Itoa(ports.OrderTableP),
			"0",
		)
		if err := cmd.Start(); err != nil {
			log.Fatal(err)
		}

	case roleSlave:
		log.Println("[PP START] ppRole = Slave")
		// TODO:

	default:
		log.Fatalln("[PP START] ppRole not 0 or 1")
	}
}

/*
switch role {
case Master:
    runActiveSystem()
    sendHeartbeatToBackup()
    sendStateToBackup()

case Slave:
    for {
        select {
        case hb := <-heartbeatChan:
            _ = hb // reset timeout
        case state := <-stateChan:
            save(state)
        case <-heartbeatTimeout:
            becomeMaster()
            startActiveSystemFromSavedState()
            spawnNewBackup()
            return
        }
    }
}
*/
