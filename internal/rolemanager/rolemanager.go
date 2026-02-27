package rolemanager

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/timer"
	"fmt"
	"log"
	"time"
)

const POLL_RATE = 1 * time.Second

/*
- ElevatorRole
- AliveList
- CurrentPrimary
*/

// TODO: EAF: this func
//func PollAliveListUpdate(reciever chan<- RoleManager, rcvBcast <-chan elev.WorldView)

func RoleManager(
	ch_Update chan elev.Elevator,
	ch_RcvAliveList chan elev.AliveList,
	ch_RoleUpdateFromRM chan elev.ElevatorRole,
	ch_HeartBeatIdToRM chan int,
	ch_SetDeadElev chan int,
	ch_AliveListUpdated chan elev.AliveList,
	ch_UpdateNumElevsFromRM chan int) {

	var elevator elev.Elevator

	ch_TimedOutId := make(chan int)

	go MonitorHeartBeats(ch_HeartBeatIdToRM, ch_TimedOutId)

	for {
		select {
		case elevator = <-ch_Update:

			if !CorrectNumElevs(elevator.NumElevs, elevator.AliveList) {
				updatedNumElevs := CountNumElevs(elevator.AliveList)
				elevator.NumElevs = updatedNumElevs
			}

			if ShouldBecomePrimary(elevator.PhysicalInfo.Id, elevator.PhysicalInfo.Role, elevator.AliveList) {
				elevator.PhysicalInfo.Role = elev.ER_Primary
				ch_RoleUpdateFromRM <- elev.ER_Primary
				ch_UpdateNumElevsFromRM <- elevator.NumElevs
			} else {
				elevator.PhysicalInfo.Role = elev.ER_Backup
				ch_RoleUpdateFromRM <- elev.ER_Backup
				ch_UpdateNumElevsFromRM <- elevator.NumElevs
			}

		case timedOutID := <-ch_TimedOutId:
			ch_SetDeadElev <- timedOutID

		case newAliveList := <-ch_AliveListUpdated:

			// TODO: Sjekk om denne heisen skal bli primary
			// TODO: oppdater NumElevs
			// TODO: Kanskje andre ting som bør gjøres med denne som trigger

			elevator.AliveList = newAliveList

			if !CorrectNumElevs(elevator.NumElevs, elevator.AliveList) {
				updatedNumElevs := CountNumElevs(elevator.AliveList)
				elevator.NumElevs = updatedNumElevs
			}

			if ShouldBecomePrimary(elevator.PhysicalInfo.Id, elevator.PhysicalInfo.Role, elevator.AliveList) {
				elevator.PhysicalInfo.Role = elev.ER_Primary
				ch_RoleUpdateFromRM <- elev.ER_Primary
				ch_UpdateNumElevsFromRM <- elevator.NumElevs
			} else {
				elevator.PhysicalInfo.Role = elev.ER_Backup
				ch_RoleUpdateFromRM <- elev.ER_Backup
				ch_UpdateNumElevsFromRM <- elevator.NumElevs
			}

		}
	}
}
func MonitorHeartBeats(ch_HeartBeatId chan int, ch_TimedOutId chan int) {

	elevTimers := make(map[int]*timer.Timer)

	for id := range ch_HeartBeatId {

		t, exists := elevTimers[id]

		if !exists {

			fmt.Printf("Ny heis oppdaget: ID %d. Starter overvåking.\n", id)
			t = timer.New(elev.HEARTBEAT_TIMEOUT)
			elevTimers[id] = t

			// Lager en ny rutine for timeren når det kommer en ny heis
			go func(id int, timeoutChan chan<- int, stopChan <-chan struct{}) {
				for {
					select {
					case <-t.C:
						timeoutChan <- id
					case <-stopChan:
						return
					}
				}
			}(id, ch_TimedOutId, t.C)
		}

		t.Start()
	}
}

func CountPrimaries(AliveList elev.AliveList) int {
	numPrimaries := 0
	for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
		if AliveList[elevId].Role == elev.ER_Primary {
			numPrimaries++
		}
	}
	return numPrimaries
}

func HasOnePrimary(AliveList elev.AliveList) bool {
	return CountPrimaries(AliveList) == 1
}

func CountNumElevs(AliveList elev.AliveList) int {
	numElevs := 0
	for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
		elevRole := AliveList[elevId].Role
		if elevRole == elev.ER_Backup || elevRole == elev.ER_Primary {
			numElevs++
		}
	}
	return numElevs
}

func CorrectNumElevs(NumElevs int, AliveList elev.AliveList) bool {
	return NumElevs == CountNumElevs(AliveList)
}

func ShouldBecomePrimary(thisId int, elevRole elev.ElevatorRole, AliveList elev.AliveList) bool {

	numPrimaries := CountPrimaries(AliveList)

	if numPrimaries > 0 {
		return false
	}

	smallestBackupId := elev.N_MAX_ELEVS + 1
	numBackups := 0

	for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
		elevRole := AliveList[elevId].Role
		if elevRole == elev.ER_Backup {
			numBackups++
			if elevId < smallestBackupId {
				smallestBackupId = elevId
			}
		}
	}

	if numBackups == 0 {
		return true
	}

	if smallestBackupId == elev.N_MAX_ELEVS+1 {
		log.Fatalln("No elevators in AliveList")
	}

	return thisId == smallestBackupId
}
