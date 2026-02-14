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
	"elevator-project-g7/internal/requests"
	"elevator-project-g7/internal/rolemanager"
	"math"
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

func OrderControl(WorldView *elev.WorldView,
	AllWorldViews *[elev.N_MAX_ELEVS]elev.WorldView,
	Ch Channels,
	NumElevs *uint8,
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

// Returnerer best heis
// Dette er bare secondary requirement i specen så tenker vi bare lager en enkel algoritme
func CalculateWhichElevator(rcvOrder elev.Order,
	OrderTable [elev.N_MAX_ELEVS][elev.N_FLOORS][elev.N_BUTTONS]elev.OrderStatus,
	AliveList []elev.ElevatorPhysicalInfo,
	NumElevs int) uint16 {

	if NumElevs == 1 {
		return AliveList[0].Id
	}

	// Large number lol
	minCost := 1000
	bestElevId := AliveList[0].Id

	for e := 0; e < NumElevs; e++ {

		currentElev := AliveList[e]
		cost := CalculateCost(rcvOrder, currentElev, OrderTable[currentElev.Id])

		if cost < minCost {
			minCost = cost
			bestElevId = currentElev.Id
		}
	}

	return bestElevId

}

// Denne beregner hvor mye det koster der heis nummer elevNum å komme seg til rcvOrder.
// Funksjonen er nok ikke optimal men sikkert bra nok :)
func CalculateCost(rcvOrder elev.Order, elevator elev.ElevatorPhysicalInfo, OrderTable [elev.N_FLOORS][elev.N_BUTTONS]elev.OrderStatus) int {

	// Cost function penalties
	penaltyFloorDiff := 3
	penaltyNumOrders := 3
	penaltyWrongDir := 10

	numOrders := 0
	OrderTableBool := orderTableToBool(OrderTable)
	orderFloor := rcvOrder.Floor
	floorDiff := int(math.Abs(float64(rcvOrder.Floor - elevator.Floor)))

	// Count num active orders for elevator
	for f := 0; f < elev.N_FLOORS; f++ {
		for b := 0; b < elev.N_BUTTONS; b++ {
			if OrderTable[f][b] == elev.OS_CONFIRMED {
				numOrders++
			}
		}
	}

	wrongDir := (orderFloor < elevator.Floor && elevator.MotorDir == elevio.MD_Up) || //Elev going up, above the order
		(orderFloor > elevator.Floor && elevator.MotorDir == elevio.MD_Down) || //Elev goign down, below the order
		(orderFloor == elevator.Floor && elevator.MotorDir == elevio.MD_Down && requests.RequestBelow(OrderTableBool, elevator.Floor)) || //Elev just went passed the order floor (down)
		(orderFloor == elevator.Floor && elevator.MotorDir == elevio.MD_Up && requests.RequestAbove(OrderTableBool, elevator.Floor)) //Elev just went passed the order floor (up)

	totalCost := penaltyFloorDiff*floorDiff + penaltyNumOrders*numOrders
	if wrongDir {
		totalCost += penaltyWrongDir
	}
	return totalCost

}

// cost++ antall floor unna target
// cost++ beveger seg i feil retning
// cost++ antall stopp (kanskje man ikke trenger så heftig logikk)

func orderTableToBool(OrderTable [elev.N_FLOORS][elev.N_BUTTONS]elev.OrderStatus) [elev.N_FLOORS][elev.N_BUTTONS]bool {
	var OrderTableBool [elev.N_FLOORS][elev.N_BUTTONS]bool

	for f := 0; f < elev.N_FLOORS; f++ {
		for b := 0; b < elev.N_BUTTONS; b++ {
			if OrderTable[f][b] == elev.OS_CONFIRMED {
				OrderTableBool[f][b] = true
			}
		}
	}

	return OrderTableBool
}
