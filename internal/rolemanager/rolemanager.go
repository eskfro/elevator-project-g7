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

			switch RM_PhysicalInfo.Role {

			case elev.ER_Dead:

			case elev.ER_Backup:
				if ShouldBecomePrimary(RM_PhysicalInfo.Id, RM_PhysicalInfo.Role, RM_NumElevs, RM_AliveList, timeStart) {
					RM_PhysicalInfo.PrimaryId = RM_PhysicalInfo.Id
					RM_PhysicalInfo.Role = elev.ER_Primary
					RM_AliveList[RM_PhysicalInfo.Id] = RM_PhysicalInfo

					ch_fromRM_Role <- elev.ER_Primary
				}
			case elev.ER_Primary:
				if ShouldBecomeBackup(RM_PhysicalInfo.Id, RM_PhysicalInfo.Role, RM_NumElevs, RM_AliveList) {
					RM_PhysicalInfo.Role = elev.ER_Backup
					RM_AliveList[RM_PhysicalInfo.Id] = RM_PhysicalInfo

					ch_fromRM_Role <- elev.ER_Backup

				}
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

func ShouldBecomePrimary(thisId int, thisRole elev.ElevatorRole, NumElevs int, AliveList elev.AliveList, timeStart time.Time) bool {

	// Wait so that it can update its AliveList. Maybe dont need this
	if time.Since(timeStart) < 200*time.Millisecond {
		return false
	}
	if CountPrimaries(AliveList) != 0 {
		return false
	}
	if NumElevs == 1 {
		return true
	}

	smallestBackupId := elev.N_MAX_ELEVS + 1

	for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
		elevRole := AliveList[elevId].Role
		if elevRole == elev.ER_Backup {
			if elevId < smallestBackupId {
				smallestBackupId = elevId
			}
		}
	}

	if smallestBackupId == elev.N_MAX_ELEVS+1 {
		log.Fatalln("No elevators in AliveList")
	}

	return thisId == smallestBackupId
}

func ShouldBecomeBackup(thisId int, thisRole elev.ElevatorRole, NumElevs int, AliveList elev.AliveList) bool {

	if NumElevs == 1 {
		return false
	}

	if CountPrimaries(AliveList) < 2 {
		return false
	}

	smallestPrimaryId := elev.N_MAX_ELEVS + 1

	for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
		if AliveList[elevId].Role == elev.ER_Primary {
			if elevId < smallestPrimaryId {
				smallestPrimaryId = elevId
			}
		}
	}

	if smallestPrimaryId == elev.N_MAX_ELEVS+1 {
		log.Fatalln("No elevators in AliveList")
	}

	return thisId != smallestPrimaryId
}
