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
	fromRX_PhysicalInfo <-chan elev.ElevatorPhysicalInfo,
	updateRM_PhysicalInfo <-chan elev.ElevatorPhysicalInfo,
	fromRM_Role chan<- elev.ElevatorRole,
	fromRM_PrimaryID chan<- int,
	fromRM_AliveList chan<- elev.AliveList,
	fromRM_ResetVersion chan<- int,
) {
	heartBeatID := make(chan int, 200)
	timedOutID := make(chan int, 50)

	// Local to RoleManager
	aliveList := initElev.AliveList
	physicalInfo := initElev.PhysicalInfo
	timeStart := time.Now()

	go monitorHeartBeats(heartBeatID, timedOutID)

	for {
		select {

		case newPhysicalInfo := <-updateRM_PhysicalInfo:
			physicalInfo = newPhysicalInfo

		case timedOutID := <-timedOutID:
			aliveList[timedOutID].Role = elev.ER_Dead
			fromRM_AliveList <- aliveList
			fromRM_ResetVersion <- timedOutID
			physicalInfo, aliveList = handleAliveListUpdate(physicalInfo, aliveList, timeStart, fromRM_AliveList, fromRM_PrimaryID, fromRM_Role, true, false)

		case heartbeat := <-fromRX_PhysicalInfo:
			select {
			case heartBeatID <- heartbeat.ID:
			default:
				log.Println("[RoleManager] Sending Heartbeat Default Case")
			}

			isHeartbeatChanged := aliveList[heartbeat.ID] != heartbeat
			isValidPrimaryID := heartbeat.PrimaryID != elev.INVALID_PRIMARY_ID
			wasDead := aliveList[heartbeat.ID].Role == elev.ER_Dead

			// I starten settes PrimaryId til INVALID
			if !isHeartbeatChanged && isValidPrimaryID && !wasDead {
				continue
			}
			if wasDead {
				fromRM_ResetVersion <- heartbeat.ID
			}
			// log.Println("[RoleManager] New AliveList update to RoleManager")
			aliveList[heartbeat.ID] = heartbeat
			fromRM_AliveList <- aliveList
			physicalInfo, aliveList = handleAliveListUpdate(physicalInfo, aliveList, timeStart, fromRM_AliveList, fromRM_PrimaryID, fromRM_Role, false, wasDead)

		}
	}
}

func handleAliveListUpdate(
	physicalInfo elev.ElevatorPhysicalInfo,
	aliveList elev.AliveList,
	timeStart time.Time,
	fromRM_AliveList chan<- elev.AliveList,
	fromRM_PrimaryID chan<- int,
	fromRM_Role chan<- elev.ElevatorRole,
	recentTimeout bool,
	wasDead bool,

) (elev.ElevatorPhysicalInfo, elev.AliveList) {

	switch physicalInfo.Role {

	case elev.ER_Dead:
		fmt.Println("[RoleManager] DEAD")
		return physicalInfo, aliveList

	case elev.ER_Backup:

		if shouldBecomePrimary(physicalInfo.ID, aliveList, timeStart) {
			log.Println("[RoleManager] Should Become Primary")
			//Set change
			physicalInfo.Role = elev.ER_Primary
			physicalInfo.PrimaryID = physicalInfo.ID
			aliveList[physicalInfo.ID] = physicalInfo

			// Send update
			fromRM_AliveList <- aliveList
			fromRM_PrimaryID <- physicalInfo.PrimaryID
			fromRM_Role <- physicalInfo.Role

			return physicalInfo, aliveList
		}

		// Update PrimaryId when we know this elevator will be a backup
		if shouldUpdatePrimaryId(aliveList, physicalInfo.PrimaryID, timeStart) {
			log.Println("[RoleManager] Should Update PrimaryID")
			newPrimaryId := getPrimaryId(aliveList, recentTimeout)
			physicalInfo.PrimaryID = newPrimaryId
			aliveList[physicalInfo.ID] = physicalInfo

			// Send update
			fromRM_AliveList <- aliveList
			fromRM_PrimaryID <- physicalInfo.PrimaryID

		}

		// If an elevator was dead we want to start events so it updates nicely on the network or something
		if wasDead {
			fromRM_AliveList <- aliveList
			fromRM_PrimaryID <- physicalInfo.PrimaryID
			fromRM_Role <- physicalInfo.Role //IDK ABOUT THIS
		}

		return physicalInfo, aliveList

	case elev.ER_Primary:

		if shouldBecomeBackup(physicalInfo.ID, aliveList) {
			log.Println("[RoleManager] Should Become Backup")
			// Set change
			physicalInfo.Role = elev.ER_Backup
			newPrimaryID := getPrimaryId(aliveList, recentTimeout)
			physicalInfo.PrimaryID = newPrimaryID
			aliveList[physicalInfo.ID] = physicalInfo

			// Send update
			fromRM_AliveList <- aliveList
			fromRM_PrimaryID <- physicalInfo.PrimaryID
			fromRM_Role <- physicalInfo.Role

		}

		if wasDead {
			fromRM_AliveList <- aliveList
			fromRM_PrimaryID <- physicalInfo.PrimaryID
			fromRM_Role <- physicalInfo.Role //IDK ABOUT THIS
		}

		return physicalInfo, aliveList
	}
	log.Println("[handleAliveListUpdate] Bottom Return Case")
	return physicalInfo, aliveList
}

func monitorHeartBeats(
	heartBeatID <-chan int,
	timedOutID chan<- int,
) {
	// Array of timers accessed by id
	var elevTimers [elev.N_MAX_ELEVS]*timer.Timer

	for id := range heartBeatID {
		isIndexInvalid := id < 0 || id >= elev.N_MAX_ELEVS

		if isIndexInvalid {
			log.Fatalf("[monitorHeartBeats] Error: ID %d out of bounds\n", id)
			continue
		}

		t := elevTimers[id]

		// INIT TIMER GOROUTINE
		if t == nil {
			fmt.Printf("[monitorHeartBeats] New elevator: ID %d. Initializing timer.\n", id)

			// Create the timer and store it in the array
			t = timer.New(elev.HEARTBEAT_TIMEOUT)
			elevTimers[id] = t

			// Start the monitoring goroutine for this specific slot
			go func(id int, timeoutChan chan<- int, timerC <-chan struct{}) {
				for range timerC {
					timeoutChan <- id
					fmt.Printf("[monitorHeartBeats] Timeout triggered on elev id = %d\n", id)
				}
			}(id, timedOutID, t.C)
		}

		t.Start()
	}
}

func getPrimaryId(aliveList elev.AliveList, recentTimeout bool) int {
	numPrimaries := countPrimaries(aliveList)
	// This is backup and should not become primary -> the lowest backupId is the new primary
	if recentTimeout && numPrimaries == 0 {
		for elevID := 0; elevID < elev.N_MAX_ELEVS; elevID++ {
			if aliveList[elevID].Role == elev.ER_Backup {
				return elevID
			}
		}
		log.Fatalln("[GetPrimaryId] AliveList Empty!")
	}

	// Else return the primary with lowest Id
	primaryID := elev.N_MAX_ELEVS
	for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
		if aliveList[elevId].Role == elev.ER_Primary {
			primaryID = elevId
			break
		}
	}
	if primaryID == elev.N_MAX_ELEVS {
		log.Fatalln("[GetPrimaryId] No Primary found in AliveList")
	}
	return primaryID

}

func countPrimaries(aliveList elev.AliveList) int {
	numPrimaries := 0
	for elevID := 0; elevID < elev.N_MAX_ELEVS; elevID++ {
		if aliveList[elevID].Role == elev.ER_Primary {
			numPrimaries++
		}
	}
	return numPrimaries
}

func CountNumElevs(aliveList elev.AliveList) int {
	numElevs := 0
	for elevID := 0; elevID < elev.N_MAX_ELEVS; elevID++ {
		isDeadElev := aliveList[elevID].Role == elev.ER_Dead
		if isDeadElev {
			continue
		}
		numElevs++
	}
	return numElevs
}

// Primary election -> the backup with smallest ID becomes primary
func shouldBecomePrimary(thisID int, aliveList elev.AliveList, timeStart time.Time) bool {
	// Make sure the alivelist is updated before we start primary election
	if time.Since(timeStart) < elev.PRIMARY_ELECTION_DELAY ||
		countPrimaries(aliveList) != 0 {
		return false
	}
	smallestBackupID := elev.N_MAX_ELEVS
	for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
		if aliveList[elevId].Role == elev.ER_Backup {
			smallestBackupID = elevId
			break
		}
	}
	if smallestBackupID == elev.N_MAX_ELEVS {
		log.Fatalln("No elevators in AliveList (Primary)")
	}
	return thisID == smallestBackupID
}

func shouldBecomeBackup(thisID int, aliveList elev.AliveList) bool {

	if countPrimaries(aliveList) < 2 {
		return false
	}
	smallestPrimaryID := elev.N_MAX_ELEVS
	for elevID := 0; elevID < elev.N_MAX_ELEVS; elevID++ {
		if aliveList[elevID].Role == elev.ER_Primary {
			smallestPrimaryID = elevID
			break
		}
	}
	if smallestPrimaryID == elev.N_MAX_ELEVS {
		log.Fatalln("[ShouldBecomeBackup] How did this even happen!")
	}
	return thisID != smallestPrimaryID
}

func shouldUpdatePrimaryId(aliveList elev.AliveList, primaryID int, timeStart time.Time) bool {

	if primaryID == elev.INVALID_PRIMARY_ID {
		return time.Since(timeStart) >= elev.PRIMARY_ELECTION_DELAY
	} else {
		return aliveList[primaryID].Role == elev.ER_Dead
	}
}
