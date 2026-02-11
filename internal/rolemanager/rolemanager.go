package rolemanager

import "time"

const POLL_RATE = 1 * time.Second

/*
- ElevatorRole
- AliveList
- CurrentPrimary
*/

type Role int

const (
	ROLE_Backup  Role = 0
	ROLE_Primary Role = 1
	ROLE_Init	 Role = 2
)

//TODO: define NumElevs, Id

func PollRoleUpdate(receiver chan<- Role) {
	prev := ROLE_Init
	for {
		time.Sleep(POLL_RATE)
		current := //TODO: Nåverande rolle
		if current != prev && v != ROLE_Init {
			receiver <- current
		}
		prev = current
	}
}
