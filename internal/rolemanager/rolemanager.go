package rolemanager

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/timer"
	"fmt"
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

func CountPrimaries(List []elev.RoleIdPair) int {
	numPrimaries := 0
	for _, pair := range List {
		if pair.Role == elev.ER_Primary {
			numPrimaries++
		}
	}
	return numPrimaries
}

// acceptance test for at det bare er en master i AliveList
func HasOnePrimary(List []elev.RoleIdPair) bool {
	return CountPrimaries(List) == 1
}

// acceptance test for at NumElevs == len(AliveList) -> kanskje unødvendig
func AT_CorrectNumElevs(NumElevs int, List []elev.RoleIdPair) bool {
	return NumElevs == len(List)
}

// lag funksjon som sjekker om en backup skal bli mastyer
func ShouldBecomePrimary(ElevatorId int, NextMasterId int, AliveList []elev.RoleIdPair) bool {

	if CountPrimaries(AliveList) != 0 {
		return false
	}

	return ElevatorId == NextMasterId
}

//TODO

/*
======== Notes for primary / backup logikk ======================

Innhold i Packet som sendes:
-> ElevatorPhysicalInfo
-> Role
-> WorldView
-> AliveList
-> Id, Port

*/
