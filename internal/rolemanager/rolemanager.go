package rolemanager

import (
	"elevator-project-g7/internal/elev"
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
	ch_RoleUpdateFromRM chan elev.ElevatorRole) {

	var elevator elev.Elevator

	for {
		select {
		case elevator = <-ch_Update:
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
