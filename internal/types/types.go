package types

import "elevator-project-g7/internal/elevio"


type ElevatorMovement int

const (
	EM_Idle     ElevatorMovement = 0
	EM_Moving   ElevatorMovement = 1
	EM_DoorOpen ElevatorMovement = 2
)

type DirnMovementPair struct {
	Direction elevio.MotorDirection
	Movement  ElevatorMovement
}