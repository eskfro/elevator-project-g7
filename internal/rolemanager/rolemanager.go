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
	ch_AliveListUpdated chan struct{}) {

	var elevator elev.Elevator
	ch_TimedOutId := make(chan int)

	go MonitorHeartBeats(ch_HeartBeatIdToRM, ch_TimedOutId)

	for {
		select {
		case elevator = <-ch_Update:

		case timedOutID := <-ch_TimedOutId:
			ch_SetDeadElev <- timedOutID

		case <-ch_AliveListUpdated:

		}
	}
}
func MonitorHeartBeats(ch_HeartBeatId chan int, ch_TimedOutId chan int) {

	elevTimers := make(map[int]*timer.Timer)

	for {
		select {
		case id := <-ch_HeartBeatId:

			t, exists := elevTimers[id]

			if !exists {

				fmt.Printf("Ny heis oppdaget: ID %d. Starter overvåking.\n", id)
				t = timer.New(elev.HEARTBEAT_TIMEOUT)
				elevTimers[id] = t

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

func ShouldBecomePrimary(_elevId int, AliveList elev.AliveList) bool {

	if CountPrimaries(AliveList) != 0 {
		return false
	}

	smallestElevId := elev.N_MAX_ELEVS + 1

	for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
		elevRole := AliveList[elevId].Role
		if elevRole == elev.ER_Backup || elevRole == elev.ER_Primary {
			if elevId < smallestElevId {
				smallestElevId = elevId
			}
		}
	}

	if smallestElevId == elev.N_MAX_ELEVS+1 {
		log.Fatalln("No elevators in AliveList")
	}

	return _elevId == smallestElevId
}
