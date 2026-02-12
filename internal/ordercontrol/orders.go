package ordercontrol

/*
- OrderTable
- PotentialOrderTable
- UnassignedOrderList P
- ClearList P
- FeasableElevatorList P
- ElevatorPhysicalInfoTable P
*/

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/rolemanager"
	"time"
)

type Channels struct {
	ButtonPress        chan elevio.ButtonEvent
	ClearOrder         chan elev.Order     //Primary
	RequestConfirmed   chan elev.Order     //Primary
	RcvBcast           chan elev.WorldView //Primary
	NewOrderRequest    chan elev.Order     //Primary
	MsgFromPrimary     chan [elev.N_MAX_ELEVS][elev.N_FLOORS][elev.N_BUTTONS]elev.OrderStatus
	RoleUpdate         chan elev.ElevatorRole
	BroadcastWorldView <-chan time.Time
}

// EAF: I MOVED THIS TO MAIN
/*
func Start(WorldView *elev.WorldView,
	AllWorldViews *[elev.N_MAX_ELEVS]elev.WorldView,
	Role *elev.ElevatorRole) {

	ticker := time.NewTicker(100 * time.Millisecond)

	Ch := Channels{
		buttonPress:      make(chan elevio.ButtonEvent),
		clearOrder:       make(chan elev.Order),     //Primary
		requestConfirmed: make(chan elev.Order),     //Primary
		rcvBcast:         make(chan elev.WorldView), //Primary		 //TODO: WorldView is not a type
		newOrderRequest:  make(chan elev.Order),     //Primary
		msgFromPrimary:   make(chan [elev.N_MAX_ELEVS][elev.N_FLOORS][elev.N_BUTTONS]elev.OrderStatus),
		roleUpdate:       make(chan elev.ElevatorRole),
		orderBcastTicker: ticker.C,
	}

	go elevio.PollButtons(Ch.buttonPress)
	go rolemanager.PollRoleUpdate(Ch.roleUpdate, *Role)
	go orderControl(WorldView, AllWorldViews, Ch)
}

*/

func OrderControl(WorldView *elev.WorldView,
	AllWorldViews *[elev.N_MAX_ELEVS]elev.WorldView,
	Ch Channels,
	NumElevs *int,
	ElevatorId *int,
	currentRole *elev.ElevatorRole) {

	for {
		switch *currentRole {
		case elev.ER_Backup:
			select {
			case *currentRole = <-Ch.RoleUpdate:

			case btnPress := <-Ch.ButtonPress:
				elevio.PrintButtonpress(btnPress)
				WorldView.OrderTable[*ElevatorId][btnPress.Floor][btnPress.Button] = elev.OS_REQUESTED

			case OrderTableFromPrimary := <-Ch.MsgFromPrimary:
				if true { //not acceptance test passed {//TODO: fix if statement
					break //TODO: maybe kill???
				}

				WorldView.OrderTable = OrderTableFromPrimary

				for floor := 0; floor < elev.N_FLOORS; floor++ {
					for btn := 0; btn < elev.N_BUTTONS; btn++ {
						WorldView.LocalOrderTable[floor][btn] = OrderTableFromPrimary[rolemanager.Id][floor][btn] == elev.OS_CONFIRMED
					}
				}

			case order := <-Ch.ClearOrder:
				WorldView.EveryonesOrders[order] = elev.OS_CLEAR

			case <-Ch.BroadcastWorldView:
				//TODO: send heile WorldView til Primary
			}

		case elev.ER_Primary:

			select {

			case *currentRole = <-Ch.RoleUpdate:

			case rcvWorldView := <-Ch.RcvBcast:
				//TODO: rcvID := Who sent the order??
				AllWorldViews[rcvID] = rcvWorldView

				for rcvOrder, rcvStatus := range rcvWorldView.EveryonesOrders { //Bytta navn fra OrderTable til EveryonesOrders, men angrer☹️
					currentStatus := WorldView.EveryonesOrders[rcvOrder]

					if isReassignable(rcvOrder, rcvStatus, currentStatus) {
						assignedID := CalculateWhichElevator(rcvOrder, AllWorldViews)
						rcvOrder.ElevatorNumber = assignedID
					}

					WorldView.EveryonesOrders[rcvOrder] = CalculateNewStatus(rcvOrder, rcvStatus, currentStatus, rcvID, *AllWorldViews, *NumElevs)
				}
			}
		}
	}
}

func isReassignable(rcvOrder elev.Order, rcvStatus elev.OrderStatus, currentStatus elev.OrderStatus) bool {
	isHallOrder := rcvOrder.ButtonType != elevio.BT_Cab
	shouldBeAssigned := (rcvStatus == elev.OS_REQUESTED) && (currentStatus == elev.OS_NO_ORDER)
	return isHallOrder && shouldBeAssigned
}

// Foreløpig ikkje ferdig funksjon
func CalculateNewStatus(rcvOrder elev.Order,
	rcvStatus elev.OrderStatus,
	currentStatus elev.OrderStatus,
	rcvID int,
	AllWorldViews [elev.N_MAX_ELEVS]elev.WorldView,
	NumElevs int) elev.OrderStatus {

	if rcvStatus == currentStatus {
		return currentStatus
	}

	isOwner := rcvID == rcvOrder.ElevatorNumber
	if isOwner {
		switch rcvStatus {

		case elev.OS_REQUESTED:
			if currentStatus == elev.OS_NO_ORDER {
				return elev.OS_REQUESTED
			}

		case elev.OS_CLEAR:
			//TODO: Make acceptance test
			return elev.OS_NO_ORDER

		case elev.OS_CONFIRMED:
			//TODO:
		}

	} else {
		switch rcvStatus {

		case elev.OS_REQUESTED:
			if AllWorldViews[rcvOrder.ElevatorNumber].EveryonesOrders[rcvOrder] != elev.OS_REQUESTED {
				// FAILED ACCEPTANCE TEST
			}
			if isRequestedByAll(AllWorldViews, rcvOrder, NumElevs) {
				return elev.OS_CONFIRMED
			}

		case elev.OS_CLEAR:
			// FAILED ACCEPTANCE TEST

		case elev.OS_CONFIRMED:
			//TODO:

		}
	}
}

func isRequestedByAll(AllWorldViews [elev.N_MAX_ELEVS]elev.WorldView, order elev.Order, NumElevs int) bool {
	for id := 0; id < NumElevs; id++ {
		if AllWorldViews[id].EveryonesOrders[order] != elev.OS_REQUESTED {
			return false
		}
	}
	return true
}
