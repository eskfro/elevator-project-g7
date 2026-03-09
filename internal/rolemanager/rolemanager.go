package rolemanager

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/timer"
	"fmt"
	"log"
	"time"
)

func RoleManager(
	elevator elev.Elevator,
	ch_updateRM_AliveList chan elev.AliveList,
	ch_updateRM_PhysicalInfo chan elev.ElevatorPhysicalInfo,
	ch_updateRM_NumElevs chan int,
	ch_fromRM_Role chan elev.ElevatorRole,
	ch_fromRM_DeadElevId chan int,
	ch_fromRM_PrimaryId chan int,
	ch_fromRM_PrimaryIp chan string,
	ch_fromRX_PhysicalInfo chan elev.ElevatorPhysicalInfo,
	ch_fromRM_AliveList chan elev.AliveList,
	ch_fromRM_NumElevs chan int,
) {

	ch_HeartBeatId := make(chan int, 50)
	ch_TimedOutId := make(chan int, 50)

	RM_AliveList := elevator.AliveList
	RM_NumElevs := elevator.NumElevs
	RM_PhysicalInfo := elevator.PhysicalInfo
	timeStart := time.Now()

	go MonitorHeartBeats(ch_HeartBeatId, ch_TimedOutId)

forLoop:
	for {
		select {

		case newAliveList := <-ch_updateRM_AliveList:
			RM_AliveList = newAliveList
			RM_PhysicalInfo, RM_AliveList, RM_NumElevs = handleAliveListUpdate(RM_PhysicalInfo, RM_AliveList, RM_NumElevs, timeStart, ch_fromRM_AliveList, ch_fromRM_PrimaryId, ch_fromRM_PrimaryIp, ch_fromRM_Role)

		case newPhysicalInfo := <-ch_updateRM_PhysicalInfo:
			RM_PhysicalInfo = newPhysicalInfo
			RM_PhysicalInfo, RM_AliveList, RM_NumElevs = handleAliveListUpdate(RM_PhysicalInfo, RM_AliveList, RM_NumElevs, timeStart, ch_fromRM_AliveList, ch_fromRM_PrimaryId, ch_fromRM_PrimaryIp, ch_fromRM_Role)

		case newNumElevs := <-ch_updateRM_NumElevs:
			RM_NumElevs = newNumElevs

		case timedOutID := <-ch_TimedOutId:
			RM_AliveList[timedOutID].Role = elev.ER_Dead
			RM_NumElevs = CountNumElevs(RM_AliveList)
			ch_fromRM_NumElevs <- RM_NumElevs
			ch_fromRM_DeadElevId <- timedOutID
			RM_PhysicalInfo, RM_AliveList, RM_NumElevs = handleAliveListUpdate(RM_PhysicalInfo, RM_AliveList, RM_NumElevs, timeStart, ch_fromRM_AliveList, ch_fromRM_PrimaryId, ch_fromRM_PrimaryIp, ch_fromRM_Role)

		// ============================================================================ HEARTBEAT RCV FROM NETWORK
		case heartbeat := <-ch_fromRX_PhysicalInfo:
			// log.Println("[RoleManager] Heartbeat from RX")
			// Always update watchdog timer
			ch_HeartBeatId <- heartbeat.Id

			isHeartbeatUnchanged := RM_AliveList[heartbeat.Id] == heartbeat
			isValidPrimaryId := heartbeat.PrimaryId != elev.INVALID_PRIMARY_ID

			//I starten settes PrimaryId til INVALID. Da bryr man seg ikke om isHeartBeatUnchanged fordi man må uansett oppdatere PrimaryId
			if isHeartbeatUnchanged && isValidPrimaryId {
				continue forLoop
			}

			// log.Println("[RoleManager] New AliveList update to RoleManager")
			RM_AliveList[heartbeat.Id] = heartbeat
			ch_fromRM_AliveList <- RM_AliveList
			RM_NumElevs = CountNumElevs(RM_AliveList)
			ch_fromRM_NumElevs <- RM_NumElevs
			RM_PhysicalInfo, RM_AliveList, RM_NumElevs = handleAliveListUpdate(RM_PhysicalInfo, RM_AliveList, RM_NumElevs, timeStart, ch_fromRM_AliveList, ch_fromRM_PrimaryId, ch_fromRM_PrimaryIp, ch_fromRM_Role)

		}
	}
}

// TODO: fiks navn på funksjoner lul
func handleAliveListUpdate(
	RM_PhysicalInfo elev.ElevatorPhysicalInfo,
	RM_AliveList elev.AliveList,
	RM_NumElevs int,
	timeStart time.Time,
	ch_fromRM_AliveList chan elev.AliveList,
	ch_fromRM_PrimaryId chan int,
	ch_fromRM_PrimaryIp chan string,
	ch_fromRM_Role chan elev.ElevatorRole,

) (elev.ElevatorPhysicalInfo, elev.AliveList, int) {

	switch RM_PhysicalInfo.Role {

	case elev.ER_Dead:
		fmt.Println("[RoleManager] DEAD")
		return RM_PhysicalInfo, RM_AliveList, RM_NumElevs

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
			ch_fromRM_PrimaryId <- RM_PhysicalInfo.PrimaryId
			ch_fromRM_PrimaryIp <- RM_PhysicalInfo.PrimaryIp
			ch_fromRM_Role <- RM_PhysicalInfo.Role

			return RM_PhysicalInfo, RM_AliveList, RM_NumElevs
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
		return RM_PhysicalInfo, RM_AliveList, RM_NumElevs

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
		return RM_PhysicalInfo, RM_AliveList, RM_NumElevs
	}
	log.Println("[handleAliveListUpdate] Bottom Return Case")
	return RM_PhysicalInfo, RM_AliveList, RM_NumElevs
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

// Primary election -> the backup with smallest ID becomes primary
func ShouldBecomePrimary(thisId int, thisRole elev.ElevatorRole, NumElevs int, AliveList elev.AliveList, timeStart time.Time) bool {
	// Make sure the alivelist is updated before we start primary election
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
