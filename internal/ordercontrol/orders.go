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
	"log"
	"math"
	"time"
)

type Channels struct {
	ButtonPress        chan elevio.ButtonEvent
	ClearOrder         chan elev.Order                                                        //Primary
	RequestConfirmed   chan elev.Order                                                        //Primary
	RcvBcast           chan [elev.N_MAX_ELEVS][elev.N_FLOORS][elev.N_BUTTONS]elev.OrderStatus //Primary
	NewOrderRequest    chan elev.Order                                                        //Primary
	MsgFromPrimary     chan [elev.N_MAX_ELEVS][elev.N_FLOORS][elev.N_BUTTONS]elev.OrderStatus
	RoleUpdate         chan elev.ElevatorRole
	BroadcastWorldView <-chan time.Time
}

func OrderControl(
	ch_Update chan elev.Elevator,
	ch_RcvOrderTable chan elev.OrderTable) { //TODO: fix {}

	var elevator elev.Elevator

	for {
		switch currentRole {
		case elev.ER_Backup:
			select {
			case elevator = <-ch_Update:

			case currentRole = <-Ch.RoleUpdate:

			case btnPress := <-Ch.ButtonPress:
				elevio.PrintButtonpress(btnPress)
				elevator.OrderTable[ElevatorId][btnPress.Floor][btnPress.Button] = elev.OS_REQUESTED

			case OrderTableFromPrimary := <-ch_RcvOrderTable:
				if true { //not acceptance test passed {//TODO: fix if statement
					break //TODO: maybe kill???
				}

				OrderTable = OrderTableFromPrimary

				for floor := 0; floor < elev.N_FLOORS; floor++ {
					for btn := 0; btn < elev.N_BUTTONS; btn++ {
						WorldView.LocalOrderTable[floor][btn] = OrderTableFromPrimary[rolemanager.Id][floor][btn] == elev.OS_CONFIRMED
					}
				}

			case order := <-Ch.ClearOrder:
				WorldView.EveryonesOrders[order] = elev.OS_CLEAR

			case elev.ER_Primary:

				select {

				case elevator = <-ch_Update:

				case *currentRole = <-Ch.RoleUpdate:

				case rcvOrderTable := <-ch_RcvOrderTable:
					//TODO: senderID := Who sent the order??

					// TODO: Send to Coordinator
					elevator.AllOrderTables[senderID] = rcvOrderTable

					for orderID := 0; orderID < int(*NumElevs); orderID++ {
						for floor := 0; floor < elev.N_FLOORS; floor++ {
							for btn := 0; floor < elev.N_BUTTONS; btn++ {

								primaryStatus := elevator.OrderTable[orderID][floor][btn]
								rcvStatus := rcvOrderTable[orderID][floor][btn]

								if isReassignable(elevio.ButtonType(btn), rcvStatus, primaryStatus) {
									orderID = CalculateWhichElevator(orderID, floor, btn, rcvOrderTable, elevator.AliveList, int(elevator.NumElevs))
								}

								elevator.OrderTable[orderID][floor][btn] = CalculateNewStatus(orderID, floor, btn, rcvStatus, primaryStatus, senderID, *AllWorldViews, *NumElevs)

							}
						}

					}
				}
			}
		}
	}
}

func isReassignable(buttonType elevio.ButtonType, rcvStatus elev.OrderStatus, currentStatus elev.OrderStatus) bool {
	isHallOrder := buttonType != elevio.BT_Cab
	shouldBeAssigned := (rcvStatus == elev.OS_REQUESTED) && (currentStatus == elev.OS_NO_ORDER)
	return isHallOrder && shouldBeAssigned
}

// Foreløpig ikkje ferdig funksjon
func CalculateNewStatus(
	orderID int,
	floor int,
	btn int,
	rcvStatus elev.OrderStatus,
	primaryStatus elev.OrderStatus,
	senderID int,
	AllOrderTables elev.AllOrderTables,
	NumElevs int) elev.OrderStatus {

	if rcvStatus == primaryStatus {
		return primaryStatus
	}

	isOwner := orderID == senderID

	if isOwner {
		switch rcvStatus {

		case elev.OS_REQUESTED:
			if primaryStatus == elev.OS_NO_ORDER {
				return elev.OS_REQUESTED
			}
			return primaryStatus

		case elev.OS_CLEAR:
			//TODO: Make acceptance test
			// Lag kode som sjekker om vi kan fjerne ordre

			return elev.OS_NO_ORDER

		case elev.OS_CONFIRMED:
			//TODO:

		}

	} else { // senderID not owner of order -> only for acks
		switch rcvStatus {

		case elev.OS_REQUESTED:
			if AllOrderTables[orderID][orderID][floor][btn] != elev.OS_REQUESTED {
				// FAILED ACCEPTANCE TEST
				log.Printf("Heis %d påstår at %d har en request i etasje %d, men det stemmer ikke i min matrise!\n", senderID, orderID, floor)
				log.Fatalln("Du kan ikkje sei at ei he requesta når ei ikkje he requesta sjølv!")
			}
			if isRequestedByAll(AllOrderTables, orderID, floor, btn, NumElevs) {
				return elev.OS_CONFIRMED
			}

		case elev.OS_CLEAR:
			// FAILED ACCEPTANCE TEST
			log.Fatalln("Kun owner kan cleare. Elev %d => selfkill", senderID)
			// TODO: kanskje drepe senderen, ikkje seg selv, idk

		case elev.OS_CONFIRMED:
			// Acceptance test
			// Trur ikkje det er noke meir
			if primaryStatus != elev.OS_CONFIRMED {
				log.Fatalln("Elev %d tried to confirm before primary confirmed => selfkill", senderID)
			}
			return elev.OS_CONFIRMED

		}
	}
}

func isRequestedByAll(AllOrderTables elev.AllOrderTables, orderID int, floor int, btn int, NumElevs int) bool {
	for i := 0; i < NumElevs; i++ {
		if AllOrderTables[i][orderID][floor][btn] != elev.OS_REQUESTED {
			return false
		}
	}
	return true
}

// Returnerer best heis
// Dette er bare secondary requirement i specen så tenker vi bare lager en enkel algoritme
func CalculateWhichElevator(
	orderId int,
	floor int,
	btn int,
	OrderTable elev.OrderTable,
	AliveList elev.AliveList,
	NumElevs int) int {

	if NumElevs == 1 {
		return AliveList[0].Id
	}

	// Large number lol
	minCost := 1000
	bestElevId := AliveList[0].Id

	for e := 0; e < NumElevs; e++ {

		currentElev := AliveList[e]

		LocalOrderTable := orderTableToBool(OrderTable[currentElev.Id])

		cost := CalculateCost(rcvOrder, currentElev, LocalOrderTable)

		if cost < minCost {
			minCost = cost
			bestElevId = currentElev.Id
		}
	}

	return bestElevId

}

// Denne beregner hvor mye det koster der heis nummer elevNum å komme seg til rcvOrder.
// Funksjonen er nok ikke optimal men sikkert bra nok :)
func CalculateCost(rcvOrder elev.Order, elevator elev.ElevatorPhysicalInfo, LocalOrderTable elev.LocalOrderTable) int {

	// Cost function penalties
	penaltyFloorDiff := 3
	penaltyNumOrders := 3
	penaltyWrongDir := 10

	numOrders := 0

	orderFloor := rcvOrder.Floor
	floorDiff := int(math.Abs(float64(rcvOrder.Floor - elevator.Floor)))

	// Count num active orders for elevator
	for f := 0; f < elev.N_FLOORS; f++ {
		for b := 0; b < elev.N_BUTTONS; b++ {
			if LocalOrderTable[f][b] {
				numOrders++
			}
		}
	}

	wrongDir := (orderFloor < elevator.Floor && elevator.MotorDir == elevio.MD_Up) || //Elev going up, above the order
		(orderFloor > elevator.Floor && elevator.MotorDir == elevio.MD_Down) || //Elev goign down, below the order
		(orderFloor == elevator.Floor && elevator.MotorDir == elevio.MD_Down && requests.RequestBelow(LocalOrderTable, elevator.Floor)) || //Elev just went passed the order floor (down)
		(orderFloor == elevator.Floor && elevator.MotorDir == elevio.MD_Up && requests.RequestAbove(LocalOrderTable, elevator.Floor)) //Elev just went passed the order floor (up)

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

	for floor := 0; floor < elev.N_FLOORS; floor++ {
		for btn := 0; btn < elev.N_BUTTONS; btn++ {
			if OrderTable[floor][btn] == elev.OS_CONFIRMED {
				OrderTableBool[floor][btn] = true
			}
		}
	}

	return OrderTableBool
}
