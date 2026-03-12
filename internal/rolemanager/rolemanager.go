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

	RM_AliveList := initElev.AliveList
	RM_PhysicalInfo := initElev.PhysicalInfo
	timeStart := time.Now()

	go MonitorHeartBeats(HeartBeatId, TimedOutId)

	for {
		select {

		case newPhysicalInfo := <-updateRM_PhysicalInfo:
			RM_PhysicalInfo = newPhysicalInfo

		case timedOutID := <-TimedOutId:
			RM_AliveList[timedOutID].Role = elev.ER_Dead
			fromRM_AliveList <- RM_AliveList
			fromRM_ResetVersion <- timedOutID
			RM_PhysicalInfo, RM_AliveList = handleAliveListUpdate(RM_PhysicalInfo, RM_AliveList, timeStart, true, fromRM_AliveList, fromRM_PrimaryId, fromRM_Role, false)

		// ============================================================================ HEARTBEAT RCV FROM NETWORK
		case heartbeat := <-fromRX_PhysicalInfo:

			// Always update watchdog timer
			select {
			case HeartBeatId <- heartbeat.Id:
			default:
				log.Println("[RoleManager] Sending Heartbeat Default Case")
			}

			isHeartbeatUnchanged := RM_AliveList[heartbeat.Id] == heartbeat
			isValidPrimaryId := heartbeat.PrimaryId != elev.INVALID_PRIMARY_ID
			wasDead := RM_AliveList[heartbeat.Id].Role == elev.ER_Dead

			//I starten settes PrimaryId til INVALID. Da bryr man seg ikke om isHeartBeatUnchanged fordi man må uansett oppdatere PrimaryId
			if isHeartbeatUnchanged && isValidPrimaryId && !wasDead {
				continue
			}

			// log.Println("[RoleManager] New AliveList update to RoleManager")
			RM_AliveList[heartbeat.Id] = heartbeat
			fromRM_AliveList <- RM_AliveList
			RM_PhysicalInfo, RM_AliveList = handleAliveListUpdate(RM_PhysicalInfo, RM_AliveList, timeStart, false, fromRM_AliveList, fromRM_PrimaryId, fromRM_Role, wasDead)

		}
	}
}

// TODO: fiks navn på funksjoner lul
func handleAliveListUpdate(
	PhysicalInfo elev.ElevatorPhysicalInfo,
	AliveList elev.AliveList,
	timeStart time.Time,
	recentTimeout bool,
	fromRM_AliveList chan elev.AliveList,
	fromRM_PrimaryId chan int,
	fromRM_Role chan elev.ElevatorRole,
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

		if wasDead {
			fromRM_AliveList <- AliveList
			fromRM_PrimaryId <- PhysicalInfo.PrimaryId //IDK ABOUT THIS
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
		return PhysicalInfo, AliveList
	}
	log.Println("[handleAliveListUpdate] Bottom Return Case")
	return PhysicalInfo, AliveList
}

func MonitorHeartBeats(HeartBeatId chan int, TimedOutId chan int) {
	// Initialize a fixed-size array of pointers to your custom Timer
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

	if recentTimeout && numPrimaries == 0 { // Return the backup with lowest elevId
		for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
			if AliveList[elevId].Role == elev.ER_Backup {
				return elevId
			}
		}
		log.Fatalln("[GetPrimaryId] AliveList Empty!")
	}

	primaryId := elev.INVALID_PRIMARY_ID
	for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
		if AliveList[elevId].Role == elev.ER_Primary {
			primaryId = elevId
			break
		}
	}
	if primaryId == elev.INVALID_PRIMARY_ID {
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
		if AliveList[elevId].Role == elev.ER_Backup && elevId < smallestBackupId {
			smallestBackupId = elevId
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
		if time.Since(timeStart) > elev.PRIMARY_ELECTION_DELAY {
			return true
		}
	} else {
		if aliveList[primaryId].Role == elev.ER_Dead {
			return true
		}
	}
	return false
}
