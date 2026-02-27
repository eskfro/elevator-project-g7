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
	ch_updateRM_AliveList chan elev.AliveList,
	ch_updateRM_PhysicalInfo chan elev.ElevatorPhysicalInfo,
	ch_updateRM_NumElevs chan int,
	ch_toRM_HeartBeatId chan int,

	ch_fromRM_Role chan elev.ElevatorRole,
	ch_fromRM_DeadElevId chan int,
	ch_fromRM_NumElevs chan int) {

	var RM_AliveList elev.AliveList
	var RM_NumElevs int
	var RM_PhysicalInfo elev.ElevatorPhysicalInfo

	timeStart := time.Now()

	ch_TimedOutId := make(chan int)

	go MonitorHeartBeats(ch_toRM_HeartBeatId, ch_TimedOutId)

	for {
		select {

		case timedOutID := <-ch_TimedOutId:
			RM_AliveList[timedOutID].Role = elev.ER_Dead
			RM_NumElevs = CountNumElevs(RM_AliveList)

			ch_fromRM_DeadElevId <- timedOutID
			ch_fromRM_NumElevs <- RM_NumElevs

		case newAliveList := <-ch_updateRM_AliveList:

			RM_AliveList = newAliveList
			RM_NumElevs = CountNumElevs(RM_AliveList)
			ch_fromRM_NumElevs <- RM_NumElevs

			if ShouldBecomePrimary(RM_PhysicalInfo.Id, RM_PhysicalInfo.Role, RM_AliveList, timeStart) {
				RM_PhysicalInfo.Role = elev.ER_Primary
				ch_fromRM_Role <- elev.ER_Primary
			} else {
				RM_PhysicalInfo.Role = elev.ER_Backup
				ch_fromRM_Role <- elev.ER_Backup

			}
		case newPhysicalInfo := <-ch_updateRM_PhysicalInfo:
			RM_PhysicalInfo = newPhysicalInfo

		case newNumElevs := <-ch_updateRM_NumElevs:
			RM_NumElevs = newNumElevs
		}
	}
}

func MonitorHeartBeats(ch_HeartBeatId chan int, ch_TimedOutId chan int) {
	elevTimers := make(map[int]*timer.Timer)

	for id := range ch_HeartBeatId {
		t, exists := elevTimers[id]

		if !exists {
			fmt.Printf("New elevator detected: ID %d. Starting monitor.\n", id)
			t = timer.New(elev.HEARTBEAT_TIMEOUT)
			elevTimers[id] = t

			// Start ONE goroutine for this elevator that lasts its lifetime
			go func(id int, timeoutChan chan<- int, timerC <-chan struct{}) {
				for {
					<-timerC // Wait for the custom timer's tick
					timeoutChan <- id
					fmt.Printf("Timeout triggered on elev id = %d\n", id)
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

func OnePrimaryExist(AliveList elev.AliveList) bool {
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

func ShouldBecomePrimary(thisId int, thisRole elev.ElevatorRole, AliveList elev.AliveList, timeStart time.Time) bool {

	if time.Since(timeStart) < 2000*time.Millisecond {
		return false
	}

	if OnePrimaryExist(AliveList) {
		if thisRole == elev.ER_Primary {
			fmt.Printf("Elevator %d should become primary\n", thisId)
			return true
		} else {
			return false
		}
	}

	smallestBackupId := elev.N_MAX_ELEVS + 1
	numBackups := 0
	numElevs := CountNumElevs(AliveList)

	for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
		elevRole := AliveList[elevId].Role
		if elevRole == elev.ER_Backup {
			numBackups++
			if elevId < smallestBackupId {
				smallestBackupId = elevId
			}
		}
	}

	if numElevs == 1 {
		return true
	}

	if smallestBackupId == elev.N_MAX_ELEVS+1 {
		log.Fatalln("No elevators in AliveList")
	}

	return thisId == smallestBackupId
}
