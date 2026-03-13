package rolemanager

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/timer"
	"fmt"
	"log"
	"time"
)

func RoleManager(
	initElev elev.Elevator,
	updateRM_PhysicalInfo chan elev.ElevatorPhysicalInfo,
	fromRM_Role chan elev.ElevatorRole,
	fromRM_PrimaryId chan int,
	fromRX_PhysicalInfo chan elev.ElevatorPhysicalInfo,
	fromRM_AliveList chan elev.AliveList,
	fromRM_ResetVersion chan int,
) {
	HeartBeatId := make(chan int, 50)
	TimedOutId := make(chan int, 50)

	// Local to RoleManager
	AliveList := initElev.AliveList
	PhysicalInfo := initElev.PhysicalInfo
	timeStart := time.Now()

	go MonitorHeartBeats(HeartBeatId, TimedOutId)

	for {
		select {

		case newPhysicalInfo := <-updateRM_PhysicalInfo:
			PhysicalInfo = newPhysicalInfo

		case timedOutID := <-TimedOutId:
			AliveList[timedOutID].Role = elev.ER_Dead
			fromRM_AliveList <- AliveList
			fromRM_ResetVersion <- timedOutID
			PhysicalInfo, AliveList = handleAliveListUpdate(PhysicalInfo, AliveList, timeStart, fromRM_AliveList, fromRM_PrimaryId, fromRM_Role, true, false)

		case heartbeat := <-fromRX_PhysicalInfo:
			// Always update watchdog timer
			select {
			case HeartBeatId <- heartbeat.Id:
			default:
				log.Println("[RoleManager] Sending Heartbeat Default Case")
			}

			isHeartbeatUnchanged := AliveList[heartbeat.Id] == heartbeat
			isValidPrimaryId := heartbeat.PrimaryId != elev.INVALID_PRIMARY_ID
			wasDead := AliveList[heartbeat.Id].Role == elev.ER_Dead

			// I starten settes PrimaryId til INVALID.
			// Da bryr man seg ikke om isHeartBeatUnchanged fordi man må uansett oppdatere PrimaryId
			if isHeartbeatUnchanged && isValidPrimaryId && !wasDead {
				continue
			}
			if wasDead {
				fromRM_ResetVersion <- heartbeat.Id
			}
			// log.Println("[RoleManager] New AliveList update to RoleManager")
			AliveList[heartbeat.Id] = heartbeat
			fromRM_AliveList <- AliveList
			PhysicalInfo, AliveList = handleAliveListUpdate(PhysicalInfo, AliveList, timeStart, fromRM_AliveList, fromRM_PrimaryId, fromRM_Role, false, wasDead)

		}
	}
}

func handleAliveListUpdate(
	PhysicalInfo elev.ElevatorPhysicalInfo,
	AliveList elev.AliveList,
	timeStart time.Time,
	fromRM_AliveList chan elev.AliveList,
	fromRM_PrimaryId chan int,
	fromRM_Role chan elev.ElevatorRole,
	recentTimeout bool,
	wasDead bool,

) (elev.ElevatorPhysicalInfo, elev.AliveList) {

	switch PhysicalInfo.Role {

	case elev.ER_Dead:
		fmt.Println("[RoleManager] DEAD")
		return PhysicalInfo, AliveList

	case elev.ER_Backup:

		if ShouldBecomePrimary(PhysicalInfo.Id, PhysicalInfo.Role, AliveList, timeStart) {
			log.Println("[RoleManager] Should Become Primary")
			//Set change
			PhysicalInfo.Role = elev.ER_Primary
			PhysicalInfo.PrimaryId = PhysicalInfo.Id
			AliveList[PhysicalInfo.Id] = PhysicalInfo

			// Send update
			fromRM_AliveList <- AliveList
			fromRM_PrimaryId <- PhysicalInfo.PrimaryId
			fromRM_Role <- PhysicalInfo.Role

			return PhysicalInfo, AliveList
		}

		// Update PrimaryId when we know this elevator will be a backup
		if ShouldUpdatePrimaryId(AliveList, PhysicalInfo.PrimaryId, timeStart) {
			log.Println("[RoleManager] Should Update PrimaryID")
			newPrimaryId := GetPrimaryId(AliveList, recentTimeout)
			PhysicalInfo.PrimaryId = newPrimaryId
			AliveList[PhysicalInfo.Id] = PhysicalInfo

			// Send update
			fromRM_AliveList <- AliveList
			fromRM_PrimaryId <- PhysicalInfo.PrimaryId

		}

		// If an elevator was dead we want to start events so it updates nicely on the network or something
		if wasDead {
			fromRM_AliveList <- AliveList
			fromRM_PrimaryId <- PhysicalInfo.PrimaryId
			fromRM_Role <- PhysicalInfo.Role //IDK ABOUT THIS
		}

		return PhysicalInfo, AliveList

	case elev.ER_Primary:

		if ShouldBecomeBackup(PhysicalInfo.Id, PhysicalInfo.Role, AliveList) {
			log.Println("[RoleManager] Should Become Backup")
			// Set change
			PhysicalInfo.Role = elev.ER_Backup
			newPrimaryId := GetPrimaryId(AliveList, recentTimeout)
			PhysicalInfo.PrimaryId = newPrimaryId
			AliveList[PhysicalInfo.Id] = PhysicalInfo

			// Send update
			fromRM_AliveList <- AliveList
			fromRM_PrimaryId <- PhysicalInfo.PrimaryId
			fromRM_Role <- PhysicalInfo.Role

		}

		if wasDead {
			fromRM_AliveList <- AliveList
			fromRM_PrimaryId <- PhysicalInfo.PrimaryId
			fromRM_Role <- PhysicalInfo.Role //IDK ABOUT THIS
		}

		return PhysicalInfo, AliveList
	}
	log.Println("[handleAliveListUpdate] Bottom Return Case")
	return PhysicalInfo, AliveList
}

func MonitorHeartBeats(HeartBeatId chan int, TimedOutId chan int) {
	// Array of timers accessed by id
	var elevTimers [elev.N_MAX_ELEVS]*timer.Timer

	for id := range HeartBeatId {
		isIndexInvalid := id < 0 || id >= elev.N_MAX_ELEVS

		if isIndexInvalid {
			log.Fatalf("[MonitorHeartBeats] Error: ID %d out of bounds\n", id)
			continue
		}

		t := elevTimers[id]

		// INIT TIMER GOROUTINE
		if t == nil {
			fmt.Printf("[MonitorHeartBeats] New elevator: ID %d. Initializing timer.\n", id)

			// Create the timer and store it in the array
			t = timer.New(elev.HEARTBEAT_TIMEOUT)
			elevTimers[id] = t

			// Start the monitoring goroutine for this specific slot
			go func(id int, timeoutChan chan<- int, timerC <-chan struct{}) {
				for range timerC {
					timeoutChan <- id
					fmt.Printf("[MonitorHeartBeats] Timeout triggered on elev id = %d\n", id)
				}
			}(id, TimedOutId, t.C)
		}

		t.Start()
	}
}

func GetPrimaryId(AliveList elev.AliveList, recentTimeout bool) int {
	numPrimaries := CountPrimaries(AliveList)
	// This is backup and should not become primary -> the lowest backupId is the new primary
	if recentTimeout && numPrimaries == 0 {
		for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
			if AliveList[elevId].Role == elev.ER_Backup {
				return elevId
			}
		}
		log.Fatalln("[GetPrimaryId] AliveList Empty!")
	}

	// Else return the primary with lowest Id
	primaryId := elev.N_MAX_ELEVS
	for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
		if AliveList[elevId].Role == elev.ER_Primary {
			primaryId = elevId
			break
		}
	}
	if primaryId == elev.N_MAX_ELEVS {
		log.Fatalln("[GetPrimaryId] No Primary found in AliveList")
	}
	return primaryId

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
		isDeadElev := AliveList[elevId].Role == elev.ER_Dead
		if isDeadElev {
			continue
		}
		numElevs++
	}
	return numElevs
}

// Primary election -> the backup with smallest ID becomes primary
func ShouldBecomePrimary(thisId int, thisRole elev.ElevatorRole, AliveList elev.AliveList, timeStart time.Time) bool {
	// Make sure the alivelist is updated before we start primary election
	if time.Since(timeStart) < elev.PRIMARY_ELECTION_DELAY ||
		CountPrimaries(AliveList) != 0 {
		return false
	}
	smallestBackupId := elev.N_MAX_ELEVS
	for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
		if AliveList[elevId].Role == elev.ER_Backup {
			smallestBackupId = elevId
			break
		}
	}
	if smallestBackupId == elev.N_MAX_ELEVS {
		log.Fatalln("No elevators in AliveList (Primary)")
	}
	return thisId == smallestBackupId
}

func ShouldBecomeBackup(thisId int, thisRole elev.ElevatorRole, AliveList elev.AliveList) bool {
	if CountPrimaries(AliveList) < 2 {
		return false
	}
	smallestPrimaryId := elev.N_MAX_ELEVS
	for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
		if AliveList[elevId].Role == elev.ER_Primary {
			smallestPrimaryId = elevId
			break
		}
	}
	if smallestPrimaryId == elev.N_MAX_ELEVS {
		log.Fatalln("[ShouldBecomeBackup] How did this even happen!")
	}
	return thisId != smallestPrimaryId
}

func ShouldUpdatePrimaryId(aliveList elev.AliveList, primaryId int, timeStart time.Time) bool {
	isInvalidPrimaryId := primaryId == elev.INVALID_PRIMARY_ID

	if isInvalidPrimaryId {
		if time.Since(timeStart) >= elev.PRIMARY_ELECTION_DELAY {
			return true
		}
	} else {
		if aliveList[primaryId].Role == elev.ER_Dead {
			return true
		}
	}
	return false
}
