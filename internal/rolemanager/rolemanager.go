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
	ch_updateRM_PhysicalInfo chan elev.ElevatorPhysicalInfo,
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

		case newPhysicalInfo := <-ch_updateRM_PhysicalInfo:
			RM_PhysicalInfo = newPhysicalInfo
			RM_PhysicalInfo, RM_AliveList, RM_NumElevs = handleAliveListUpdate(RM_PhysicalInfo, RM_AliveList, RM_NumElevs, timeStart, ch_fromRM_AliveList, ch_fromRM_PrimaryId, ch_fromRM_PrimaryIp, ch_fromRM_Role, ch_fromRM_NumElevs)

		case timedOutID := <-ch_TimedOutId:
			RM_AliveList[timedOutID].Role = elev.ER_Dead
			RM_NumElevs = CountNumElevs(RM_AliveList)
			ch_fromRM_DeadElevId <- timedOutID
			RM_PhysicalInfo, RM_AliveList, RM_NumElevs = handleAliveListUpdate(RM_PhysicalInfo, RM_AliveList, RM_NumElevs, timeStart, ch_fromRM_AliveList, ch_fromRM_PrimaryId, ch_fromRM_PrimaryIp, ch_fromRM_Role, ch_fromRM_NumElevs)

		// ============================================================================ HEARTBEAT RCV FROM NETWORK
		case heartbeat := <-ch_fromRX_PhysicalInfo:
			//log.Printf("[RoleManager] Heartbeat from RX | id = %d\n", heartbeat.Id)
			// Always update watchdog timer
			select {
			case ch_HeartBeatId <- heartbeat.Id:
			default:
				log.Println("[RoleManager] Sending Heartbeat Default Case")
			}

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
			RM_PhysicalInfo, RM_AliveList, RM_NumElevs = handleAliveListUpdate(RM_PhysicalInfo, RM_AliveList, RM_NumElevs, timeStart, ch_fromRM_AliveList, ch_fromRM_PrimaryId, ch_fromRM_PrimaryIp, ch_fromRM_Role, ch_fromRM_NumElevs)

		}
	}
}

// TODO: fiks navn på funksjoner lul
func handleAliveListUpdate(
	PhysicalInfo elev.ElevatorPhysicalInfo,
	AliveList elev.AliveList,
	NumElevs int,
	timeStart time.Time,
	ch_fromRM_AliveList chan elev.AliveList,
	ch_fromRM_PrimaryId chan int,
	ch_fromRM_PrimaryIp chan string,
	ch_fromRM_Role chan elev.ElevatorRole,
	ch_fromRM_NumElevs chan int,

) (elev.ElevatorPhysicalInfo, elev.AliveList, int) {

	switch PhysicalInfo.Role {

	case elev.ER_Dead:
		fmt.Println("[RoleManager] DEAD")
		return PhysicalInfo, AliveList, NumElevs

	case elev.ER_Backup:

		if ShouldBecomePrimary(PhysicalInfo.Id, PhysicalInfo.Role, NumElevs, AliveList, timeStart) {
			log.Println("[RoleManager] Should Become Primary")
			//Set change
			PhysicalInfo.Role = elev.ER_Primary
			PhysicalInfo.PrimaryIp = PhysicalInfo.Ip
			PhysicalInfo.PrimaryId = PhysicalInfo.Id
			AliveList[PhysicalInfo.Id] = PhysicalInfo
			NumElevs = CountNumElevs(AliveList)

			// Send update
			ch_fromRM_AliveList <- AliveList
			ch_fromRM_PrimaryId <- PhysicalInfo.PrimaryId
			ch_fromRM_PrimaryIp <- PhysicalInfo.PrimaryIp
			ch_fromRM_Role <- PhysicalInfo.Role
			ch_fromRM_NumElevs <- NumElevs

			return PhysicalInfo, AliveList, NumElevs
		}

		// Update PrimaryId when we know this elevator will be a backup
		if ShouldUpdatePrimaryId(PhysicalInfo.PrimaryId, timeStart) {
			log.Println("[RoleManager] Should Update PrimaryID")
			// Set change
			newPrimaryId := GetPrimaryId(AliveList)
			newPrimaryIp := GetPrimaryIp(AliveList)
			PhysicalInfo.PrimaryId = newPrimaryId
			PhysicalInfo.PrimaryIp = newPrimaryIp
			AliveList[PhysicalInfo.Id] = PhysicalInfo
			NumElevs = CountNumElevs(AliveList)

			// Send update
			ch_fromRM_AliveList <- AliveList
			ch_fromRM_PrimaryId <- PhysicalInfo.PrimaryId
			ch_fromRM_PrimaryIp <- PhysicalInfo.PrimaryIp
			ch_fromRM_NumElevs <- NumElevs

		}
		return PhysicalInfo, AliveList, NumElevs

	case elev.ER_Primary:

		if ShouldBecomeBackup(PhysicalInfo.Id, PhysicalInfo.Role, NumElevs, AliveList) {
			log.Println("[RoleManager] Should Become Backup")
			// Set change
			PhysicalInfo.Role = elev.ER_Backup
			newPrimaryId := GetPrimaryId(AliveList)
			newPrimaryIp := GetPrimaryIp(AliveList)
			PhysicalInfo.PrimaryId = newPrimaryId
			PhysicalInfo.PrimaryIp = newPrimaryIp
			AliveList[PhysicalInfo.Id] = PhysicalInfo
			NumElevs = CountNumElevs(AliveList)

			// Send update
			ch_fromRM_AliveList <- AliveList
			ch_fromRM_PrimaryId <- PhysicalInfo.PrimaryId
			ch_fromRM_PrimaryIp <- PhysicalInfo.PrimaryIp
			ch_fromRM_Role <- PhysicalInfo.Role
			ch_fromRM_NumElevs <- NumElevs

		}
		return PhysicalInfo, AliveList, NumElevs
	}
	log.Println("[handleAliveListUpdate] Bottom Return Case")
	return PhysicalInfo, AliveList, NumElevs
}

func MonitorHeartBeats(ch_HeartBeatId chan int, ch_TimedOutId chan int) {
	// Initialize a fixed-size array of pointers to your custom Timer
	var elevTimers [elev.N_MAX_ELEVS]*timer.Timer

	for id := range ch_HeartBeatId {
		isIndexInvalid := id < 0 || id >= elev.N_MAX_ELEVS

		if isIndexInvalid {
			fmt.Printf("[MonitorHeartBeats] Error: ID %d out of bounds\n", id)
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
