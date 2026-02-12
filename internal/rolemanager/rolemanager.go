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

// Structen som skal være i AliveList
type RoleIdPair struct {
	Id   int
	Role elev.ElevatorRole
}

/*
type RoleManager struct {
	NumElevs  int
	AliveList []RoleIdPair
}
*/

// TODO: EAF: this func
//func PollAliveListUpdate(reciever chan<- RoleManager, rcvBcast <-chan elev.WorldView)

func PollRoleUpdate(receiver chan<- elev.ElevatorRole, role *elev.ElevatorRole) {
	prev := elev.ER_Init
	for {
		time.Sleep(POLL_RATE)
		current := *role //TODO: Check if Marius agree
		if current != prev && *role != elev.ER_Init {
			receiver <- current
		}
		prev = current
	}
}

// Teller # master i AliveList
func CountMasterInAliveList(AliveList []RoleIdPair) int {
	numMaster := 0
	length := len(AliveList)

	for i := 0; i < length; i++ {
		if AliveList[i].Role == elev.ER_Primary {
			numMaster++
		}
	}
	return numMaster
}

// acceptance test for at det bare er en master i AliveList
func AT_OneMaster(AliveList []RoleIdPair) bool {
	return CountMasterInAliveList(AliveList) == 1
}

// acceptance test for at NumElevs == len(AliveList) -> kanskje unødvendig
func AT_CorrectNumElevs(NumElevs int, AliveList []RoleIdPair) bool {
	return NumElevs == len(AliveList)
}

// lag funksjon som sjekker om en backup skal bli mastyer
func ShouldBecomeMaster(ElevatorId int, NextMasterId int, AliveList []RoleIdPair) bool {

	if CountMasterInAliveList(AliveList) != 0 {
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
