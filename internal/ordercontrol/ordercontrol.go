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
	"fmt"
	"math"
)

func OrderControl(
	initElev elev.Elevator,
	ch_updateOC_OrderTable chan elev.OrderTable,
	ch_updateOC_AllOrderTables chan elev.AllOrderTables,
	ch_updateOC_PhysicalInfo chan elev.ElevatorPhysicalInfo,
	ch_updateOC_AliveList chan elev.AliveList,
	ch_updateOC_NumElevs chan int,
	ch_RxOrderTableP chan elev.OrderTablePacket,
	ch_fromOC_LOT chan elev.LocalOrderTable,
	ch_fromOC_OrderTable chan elev.OrderTable) {

	OC_OrderTable := initElev.OrderTable
	OC_PrevOrderTable := initElev.OrderTable
	OC_AllOrderTables := initElev.AllOrderTables
	OC_PhysicalInfo := initElev.PhysicalInfo
	OC_AliveList := initElev.AliveList
	OC_NumElevs := initElev.NumElevs

	for {

		select {

		case newOrderTable := <-ch_updateOC_OrderTable:

			OC_OrderTable = newOrderTable

			/*
				CODE FOR SINGLE ELEVATOR
				OC_PhysicalInfo.LocalOrderTable = orderTableToLOT(OC_OrderTable, OC_PhysicalInfo.Id)
				ch_fromOC_LOT <- OC_PhysicalInfo.LocalOrderTable
			*/

		case newAllOrderTables := <-ch_updateOC_AllOrderTables:

			OC_AllOrderTables = newAllOrderTables

			// TODO: sjekk om man can confirme requested ordre hvis alle har tilstand OS_Requested

		case newPhysicalInfo := <-ch_updateOC_PhysicalInfo:

			OC_PhysicalInfo = newPhysicalInfo

		case newAliveList := <-ch_updateOC_AliveList:

			OC_AliveList = newAliveList

		case newNumElevs := <-ch_updateOC_NumElevs:

			OC_NumElevs = newNumElevs

		case packet := <-ch_RxOrderTableP:

			if packet.OrderTable == OC_PrevOrderTable {
				break
			}
			OC_PrevOrderTable = packet.OrderTable

			switch OC_PhysicalInfo.Role {

			case elev.ER_Backup:

				// Break if backup and message not from primary
				if packet.Id != OC_PhysicalInfo.PrimaryId {
					break
				}

				// Oppdater backup sine verdier ihht primary
				OC_OrderTable = packet.OrderTable
				ch_fromOC_OrderTable <- OC_OrderTable
				ch_fromOC_LOT <- orderTableToLOT(OC_OrderTable, OC_PhysicalInfo.Id)

			case elev.ER_Primary:

				newOrderTable := packet.OrderTable
				OC_AllOrderTables[packet.Id] = newOrderTable

				for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {
					for floor := 0; floor < elev.N_FLOORS; floor++ {
						for btn := 0; btn < elev.N_BUTTONS; btn++ {

							primaryStatus := OC_OrderTable[elevIndex][floor][btn]
							rcvStatus := newOrderTable[elevIndex][floor][btn]

							if isRequestedByAll(OC_AllOrderTables, elevIndex, floor, btn, OC_AliveList) {
								OC_OrderTable[elevIndex][floor][btn] = elev.OS_CONFIRMED

							} else if isClearedByAll(OC_AllOrderTables, elevIndex, floor, btn, OC_AliveList) {
								OC_OrderTable[elevIndex][floor][btn] = elev.OS_NO_ORDER

							} else if isReassignable(elevio.ButtonType(btn), rcvStatus, primaryStatus) {
								bestID := CalculateWhichElevator(elevIndex, floor, btn, newOrderTable, OC_AliveList, int(OC_NumElevs))
								fmt.Printf("isReassignable: bestID = %d\n", bestID)
								OC_OrderTable[bestID][floor][btn] = elev.OS_REQUESTED

							} else {
								OC_OrderTable[elevIndex][floor][btn] = CalculateNewStatus(elevIndex, floor, btn, rcvStatus, primaryStatus, packet.Id, OC_AllOrderTables, OC_AliveList, OC_PhysicalInfo.Id)
							}

						}
					}
				}
				OC_PhysicalInfo.LocalOrderTable = orderTableToLOT(OC_OrderTable, OC_PhysicalInfo.Id)
				ch_fromOC_OrderTable <- OC_OrderTable
				ch_fromOC_LOT <- OC_PhysicalInfo.LocalOrderTable

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
	elevIndex int,
	floor int,
	btn int,
	rcvStatus elev.OrderStatus,
	primaryStatus elev.OrderStatus,
	packetID int,
	AllOrderTables elev.AllOrderTables,
	AliveList elev.AliveList,
	thisID int) elev.OrderStatus {

	switch rcvStatus {

	case elev.OS_NO_ORDER:

		if primaryStatus == elev.OS_NO_ORDER {
			return elev.OS_NO_ORDER
		} else if primaryStatus == elev.OS_CLEAR && packetID == thisID {
			return elev.OS_CLEAR
		} else {
			fmt.Println("Bindestrek 1")
			return elev.OS_NO_ORDER

		}

	case elev.OS_CONFIRMED:

		if primaryStatus == elev.OS_REQUESTED && packetID == thisID || primaryStatus == elev.OS_CONFIRMED {
			return elev.OS_CONFIRMED
		} else {
			fmt.Println("Bindestrek 3")
			return elev.OS_NO_ORDER

		}

	case elev.OS_CLEAR:

		if primaryStatus == elev.OS_CONFIRMED && packetID == elevIndex {
			return elev.OS_CLEAR
		} else {
			fmt.Println("Bindestrek 4")
			return elev.OS_NO_ORDER

		}

	default:
		fmt.Println("CalculateNewStatus failed: Kraftig Bindestrek")
		return elev.OS_NO_ORDER
	}

}

func isRequestedByAll(
	AllOrderTables elev.AllOrderTables,
	orderID int,
	floor int,
	btn int,
	AliveList elev.AliveList) bool {
	for i := 0; i < elev.N_MAX_ELEVS; i++ {

		if AliveList[i].Role == elev.ER_Dead {
			continue
		}

		if AllOrderTables[i][orderID][floor][btn] != elev.OS_REQUESTED {
			return false
		}
	}
	return true
}

func isClearedByAll(
	AllOrderTables elev.AllOrderTables,
	orderID int,
	floor int,
	btn int,
	AliveList elev.AliveList) bool {
	for i := 0; i < elev.N_MAX_ELEVS; i++ {

		if AliveList[i].Role == elev.ER_Dead {
			continue
		}

		if AllOrderTables[i][orderID][floor][btn] != elev.OS_CLEAR {
			return false
		}
	}
	return true
}

// Returnerer best heis
// Dette er bare secondary requirement i specen så tenker vi bare lager en enkel algoritme
func CalculateWhichElevator(
	orderId int,
	orderFloor int,
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

	for i := 0; i < elev.N_MAX_ELEVS; i++ {

		if AliveList[i].Role == elev.ER_Dead {
			continue
		}

		currentElev := AliveList[i]

		LocalOrderTable := orderTableToLOT(OrderTable, currentElev.Id)

		cost := CalculateCost(orderFloor, currentElev, LocalOrderTable)

		if cost < minCost {
			minCost = cost
			bestElevId = currentElev.Id
		}
	}

	return bestElevId

}

// Denne beregner hvor mye det koster der heis nummer elevNum å komme seg til rcvOrder.
// Funksjonen er nok ikke optimal men sikkert bra nok :)
func CalculateCost(orderFloor int, elevator elev.ElevatorPhysicalInfo, LocalOrderTable elev.LocalOrderTable) int {

	// Cost function penalties
	penaltyFloorDiff := 3
	penaltyNumOrders := 3
	penaltyWrongDir := 10

	numOrders := 0

	floorDiff := int(math.Abs(float64(orderFloor - elevator.Floor)))

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
		(orderFloor == elevator.Floor && elevator.MotorDir == elevio.MD_Down && requests.RequestBelow(LocalOrderTable, elevator.Floor)) || //Elev just went past the order floor (down)
		(orderFloor == elevator.Floor && elevator.MotorDir == elevio.MD_Up && requests.RequestAbove(LocalOrderTable, elevator.Floor)) //Elev just went past the order floor (up)

	totalCost := penaltyFloorDiff*floorDiff + penaltyNumOrders*numOrders
	if wrongDir {
		totalCost += penaltyWrongDir
	}
	return totalCost

}

// cost++ antall floor unna target
// cost++ beveger seg i feil retning
// cost++ antall stopp (kanskje man ikke trenger så heftig logikk)

func orderTableToLOT(OrderTable elev.OrderTable, elevId int) elev.LocalOrderTable {

	var LOT elev.LocalOrderTable

	for f := 0; f < elev.N_FLOORS; f++ {
		for b := 0; b < elev.N_BUTTONS; b++ {
			LOT[f][b] = OrderTable[elevId][f][b] == elev.OS_CONFIRMED
		}
	}

	return LOT
}
