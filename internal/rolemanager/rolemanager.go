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
	elevator elev.Elevator,
	ch_updateRM_AliveList chan elev.AliveList,
	ch_updateRM_PhysicalInfo chan elev.ElevatorPhysicalInfo,
	ch_updateRM_NumElevs chan int,
	ch_fromRM_Role chan elev.ElevatorRole,
	ch_fromRM_DeadElevId chan int,
	ch_fromRM_NumElevs chan int,
	ch_fromRM_PrimaryId chan int,
	ch_fromRM_PrimaryIp chan string,
	ch_fromRX_PhysicalInfo chan elev.ElevatorPhysicalInfo,
	ch_fromRM_AliveList chan elev.AliveList,
) {

	ch_HeartBeatId := make(chan int, 50)

	RM_AliveList := elevator.AliveList
	RM_NumElevs := elevator.NumElevs
	RM_PhysicalInfo := elevator.PhysicalInfo
	timeStart := time.Now()

	ch_TimedOutId := make(chan int, 50)

	go MonitorHeartBeats(ch_HeartBeatId, ch_TimedOutId)

	for {
		select {

		case newAliveList := <-ch_updateRM_AliveList:
			RM_AliveList = newAliveList
			ch_fromRX_PhysicalInfo <- RM_PhysicalInfo

		case newPhysicalInfo := <-ch_updateRM_PhysicalInfo:
			RM_PhysicalInfo = newPhysicalInfo
			ch_fromRX_PhysicalInfo <- RM_PhysicalInfo

		case newNumElevs := <-ch_updateRM_NumElevs:
			RM_NumElevs = newNumElevs

		case timedOutID := <-ch_TimedOutId:
			RM_AliveList[timedOutID].Role = elev.ER_Dead
			RM_NumElevs = CountNumElevs(RM_AliveList)
			ch_fromRM_DeadElevId <- timedOutID
			ch_fromRM_NumElevs <- RM_NumElevs
			//ch_fromRM_AliveList <- RM_AliveList // TODO: Se her

		// ============================================================================ HEARTBEAT RCV FROM NETWORK
		case heartbeat := <-ch_fromRX_PhysicalInfo:
			log.Println("[RoleManager] Heartbeat from RX")
			// Always update watchdog timer
			ch_HeartBeatId <- heartbeat.Id

			if RM_AliveList[heartbeat.Id] == RM_PhysicalInfo && heartbeat.PrimaryId != elev.INVALID_PRIMARY_ID {
				break
			}

			log.Println("[RoleManager] New AliveList update to RoleManager")
			RM_AliveList[heartbeat.Id] = heartbeat
			ch_fromRM_AliveList <- RM_AliveList
			RM_NumElevs = CountNumElevs(RM_AliveList)
			// if newNumElevs != RM_NumElevs {
			// 	ch_fromRM_NumElevs <- newNumElevs
			// }

			switch RM_PhysicalInfo.Role {

			case elev.ER_Dead:
				fmt.Println("[RoleManager] DEAD")

			case elev.ER_Backup:

				if ShouldBecomePrimary(RM_PhysicalInfo.Id, RM_PhysicalInfo.Role, RM_NumElevs, RM_AliveList, timeStart) {
					log.Println("[RoleManager] Should Become Primary")
					//Set change
					RM_PhysicalInfo.Role = elev.ER_Primary
					RM_PhysicalInfo.PrimaryIp = RM_PhysicalInfo.Ip
					RM_PhysicalInfo.PrimaryId = RM_PhysicalInfo.Id
					RM_AliveList[RM_PhysicalInfo.Id] = RM_PhysicalInfo

					// Send update
					ch_fromRM_AliveList <- RM_AliveList
					ch_fromRM_Role <- RM_PhysicalInfo.Role
					ch_fromRM_PrimaryId <- RM_PhysicalInfo.PrimaryId
					ch_fromRM_PrimaryIp <- RM_PhysicalInfo.PrimaryIp
				}

				// Update PrimaryId when we know this elevator will be a backup
				if ShouldUpdatePrimaryId(RM_PhysicalInfo.PrimaryId, timeStart) {
					log.Println("[RoleManager] Should Update PrimaryID")
					// Set change
					newPrimaryId := GetPrimaryId(RM_AliveList)
					newPrimaryIp := GetPrimaryIp(RM_AliveList)
					RM_PhysicalInfo.PrimaryId = newPrimaryId
					RM_PhysicalInfo.PrimaryIp = newPrimaryIp
					RM_AliveList[RM_PhysicalInfo.Id] = RM_PhysicalInfo

					// Send update
					ch_fromRM_AliveList <- RM_AliveList
					ch_fromRM_PrimaryId <- RM_PhysicalInfo.PrimaryId
					ch_fromRM_PrimaryIp <- RM_PhysicalInfo.PrimaryIp

				}

			case elev.ER_Primary:

				if ShouldBecomeBackup(RM_PhysicalInfo.Id, RM_PhysicalInfo.Role, RM_NumElevs, RM_AliveList) {
					log.Println("[RoleManager] Should Become Backup")
					// Set change
					RM_PhysicalInfo.Role = elev.ER_Backup
					newPrimaryId := GetPrimaryId(RM_AliveList)
					newPrimaryIp := GetPrimaryIp(RM_AliveList)
					RM_PhysicalInfo.PrimaryId = newPrimaryId
					RM_PhysicalInfo.PrimaryIp = newPrimaryIp
					RM_AliveList[RM_PhysicalInfo.Id] = RM_PhysicalInfo

					// Send update
					ch_fromRM_AliveList <- RM_AliveList
					ch_fromRM_PrimaryId <- RM_PhysicalInfo.PrimaryId
					ch_fromRM_PrimaryIp <- RM_PhysicalInfo.PrimaryIp
					ch_fromRM_Role <- RM_PhysicalInfo.Role

				}
			}

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

func GetPrimaryId(AliveList elev.AliveList) int {
	primaryId := elev.INVALID_ELEVATOR_ID
	for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
		if AliveList[elevId].Role == elev.ER_Primary {
			primaryId = elevId
			break
		}
	}
	if primaryId == elev.INVALID_ELEVATOR_ID {
		log.Fatalf("GetPrimaryId failed: No primary in AliveList\n")
	}
	return primaryId

}

func GetPrimaryIp(AliveList elev.AliveList) string {
	primaryIp := elev.INVALID_PRIMARY_IP
	for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
		if AliveList[elevId].Role == elev.ER_Primary {
			primaryIp = AliveList[elevId].Ip
			break
		}
	}
	if primaryIp == elev.INVALID_PRIMARY_IP {
		log.Fatalf("GetPrimaryIp failed: No primary in AliveList\n")
	}
	return primaryIp

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

	if time.Since(timeStart) < elev.PRIMARY_ELECTION_DELAY ||
		CountPrimaries(AliveList) != 0 {
		return false
	}
	if NumElevs == 1 {
		return true
	}
	smallestBackupId := elev.INVALID_ELEVATOR_ID
	for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
		if AliveList[elevId].Role == elev.ER_Backup && elevId < smallestBackupId {
			smallestBackupId = elevId
		}
	}

	if smallestBackupId == elev.INVALID_ELEVATOR_ID {
		log.Fatalln("No elevators in AliveList (Primary)")
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

	smallestPrimaryId := elev.INVALID_ELEVATOR_ID

	for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
		if AliveList[elevId].Role == elev.ER_Primary && elevId < smallestPrimaryId {
			smallestPrimaryId = elevId

		}
	}

	if smallestPrimaryId == elev.INVALID_ELEVATOR_ID {
		log.Fatalln("No elevators in AliveList (Backup)")
	}

	return thisId != smallestPrimaryId
}

func ShouldUpdatePrimaryId(primaryId int, timeStart time.Time) bool {
	return primaryId == elev.INVALID_ELEVATOR_ID && time.Since(timeStart) > elev.PRIMARY_ELECTION_DELAY
}
