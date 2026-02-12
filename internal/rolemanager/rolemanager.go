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

type ElevatorRole int

const (
	ER_Backup  ElevatorRole = 0
	ER_Primary ElevatorRole = 1
	ER_Init    ElevatorRole = 2
)

type RoleIdPair struct {
	Id   int
	Role ElevatorRole
}

//TODO: define NumElevs, Id

type RoleManager struct {
	NumElevs  int
	AliveList RoleIdPair
}

// TODO: EAF: this func
func PollAliveListUpdate(reciever chan<- RoleManager, rcvBcast <-chan elev.WorldView)

func PollRoleUpdate(receiver chan<- ElevatorRole, role ElevatorRole) {
	prev := ER_Init
	for {
		time.Sleep(POLL_RATE)
		current := role //TODO: Check if Marius agree
		if current != prev && role != ER_Init {
			receiver <- current
		}
		prev = current
	}
}
