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
	"log"
	"math"
)

func OrderControl(
	initElev elev.Elevator,
	ch_updateOC_AllOrderTables chan elev.AllOrderTables,
	ch_updateOC_PhysicalInfo chan elev.ElevatorPhysicalInfo,
	ch_updateOC_AliveList chan elev.AliveList,
	ch_updateOC_NumElevs chan int,
	ch_fromRX_OrderTableP chan elev.OrderTablePacket,
	ch_fromOC_LOT chan elev.LocalOrderTable,
	ch_fromOC_OrderTable chan elev.OrderTable,
	ch_toOC_PrimaryOrderTableP chan elev.OrderTablePacket) {

	// Local to OrderControl
	OC_OrderTable := initElev.OrderTable
	OC_PrevOrderTable := initElev.OrderTable
	OC_AllOrderTables := initElev.AllOrderTables

	OC_PhysicalInfo := initElev.PhysicalInfo
	OC_AliveList := initElev.AliveList
	OC_NumElevs := initElev.NumElevs

	for {
		// sletta ch_updateOC_Ordertable, det var ingen som skreiv til den, og trur den var farlig
		select {

		case newAllOrderTables := <-ch_updateOC_AllOrderTables:
			OC_AllOrderTables = newAllOrderTables

		case newPhysicalInfo := <-ch_updateOC_PhysicalInfo:
			OC_PhysicalInfo = newPhysicalInfo

		case newAliveList := <-ch_updateOC_AliveList:
			OC_AliveList = newAliveList

		//Delete case later
		case newNumElevs := <-ch_updateOC_NumElevs:
			OC_NumElevs = newNumElevs

		// ====== PRIMARY UPDATES ITSELF ===============
		case packetPrimary := <-ch_toOC_PrimaryOrderTableP:

			// Update data from its own packet
			OC_OrderTable = packetPrimary.OrderTable
			OC_AllOrderTables[packetPrimary.Id] = packetPrimary.OrderTable

			newPrimaryOrderTable := CalculateNewPrimaryOrderTable(OC_OrderTable, OC_AliveList, OC_AllOrderTables, OC_PhysicalInfo, OC_NumElevs, packetPrimary)

			// Update local info
			OC_OrderTable = newPrimaryOrderTable
			OC_PrevOrderTable = OC_OrderTable
			OC_AllOrderTables[OC_PhysicalInfo.Id] = OC_OrderTable
			OC_PhysicalInfo.LocalOrderTable = orderTableToLOT(OC_OrderTable, OC_PhysicalInfo.Id)

			// Send info to main
			ch_fromOC_LOT <- OC_PhysicalInfo.LocalOrderTable
			ch_fromOC_OrderTable <- OC_OrderTable

		// ====== RX FROM NETWORK =============
		case packet := <-ch_fromRX_OrderTableP:
			switch OC_PhysicalInfo.Role {

			// =========== BACKUP RCV NETWORK PACKET ================
			case elev.ER_Backup:
				// Ignore dersom ingen endring i packet sin OrderTable
				if packet.OrderTable == OC_PrevOrderTable { //	Flytta "if no update" inn i kvar rolle. Som backup var den riktig, men primary må sjekke riktig heis sin orderTable. Sjå 🏷️
					break
				}
				// TODO: kanskje oppdater OC_PrevOrderTable
				// Oppdater backup sitt OrderTable dersom packet er fra primary
				if packet.Id == OC_PhysicalInfo.PrimaryId {
					OC_PrevOrderTable = packet.OrderTable
					OC_OrderTable = packet.OrderTable
					OC_PhysicalInfo.LocalOrderTable = orderTableToLOT(OC_OrderTable, OC_PhysicalInfo.Id)
					ch_fromOC_OrderTable <- OC_OrderTable
					ch_fromOC_LOT <- OC_PhysicalInfo.LocalOrderTable
				}

			// ========== PRIMARY RCV NETWORK PACKET ========================
			case elev.ER_Primary:

				// Ignorer dersom det er ingen endringer
				if OC_AllOrderTables[packet.Id] == packet.OrderTable { // 🏷️
					break
				}
				OC_AllOrderTables[packet.Id] = packet.OrderTable

				newOrderTable := CalculateNewPrimaryOrderTable(OC_OrderTable, OC_AliveList, OC_AllOrderTables, OC_PhysicalInfo, OC_NumElevs, packet)
				// Ignorer dersom det er ingen endringer
				if OC_OrderTable == newOrderTable {
					break
				}
				OC_OrderTable = newOrderTable
				OC_PrevOrderTable = OC_OrderTable
				OC_AllOrderTables[OC_PhysicalInfo.Id] = OC_OrderTable
				OC_PhysicalInfo.LocalOrderTable = orderTableToLOT(OC_OrderTable, OC_PhysicalInfo.Id)
				ch_fromOC_OrderTable <- OC_OrderTable
				ch_fromOC_LOT <- OC_PhysicalInfo.LocalOrderTable
				//elev.PrintOrderTableSlice(OC_OrderTable, packet.Id)

			}
		}
	}
}

func CalculateNewPrimaryOrderTable(
	OC_OrderTable elev.OrderTable,
	OC_AliveList elev.AliveList,
	OC_AllOrderTables elev.AllOrderTables,
	OC_PhysicalInfo elev.ElevatorPhysicalInfo,
	OC_NumElevs int,
	packet elev.OrderTablePacket) elev.OrderTable {

	primaryOT := OC_OrderTable
	rcvOT := packet.OrderTable

	// Iterate through Primaries OrderTable and the Recieved OrderTable
	for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {
		// Skip if elevator is not on the network (ER_Dead)
		if OC_AliveList[elevIndex].Role == elev.ER_Dead {
			continue
		}
		for floor := 0; floor < elev.N_FLOORS; floor++ {
			for btn := 0; btn < elev.N_BUTTONS; btn++ {
				// Define helpers
				primaryStatus := primaryOT[elevIndex][floor][btn]
				rcvStatus := rcvOT[elevIndex][floor][btn]

				// Handle cab calls
				if elevio.ButtonType(btn) == elevio.BT_Cab {
					if rcvStatus == elev.OS_REQUESTED {
						primaryOT[elevIndex][floor][btn] = elev.OS_CONFIRMED
						continue
					}
					if rcvStatus == elev.OS_CLEAR {
						primaryOT[elevIndex][floor][btn] = elev.OS_NO_ORDER
						continue
					}
				}

				// Request trigger
				if primaryStatus == elev.OS_NO_ORDER {
					if rcvStatus == elev.OS_REQUESTED {
						primaryOT[elevIndex][floor][btn] = elev.OS_REQUESTED
						continue
					}
				}

				if primaryStatus == elev.OS_CLEAR {
					if rcvStatus == elev.OS_REQUESTED {
						primaryOT[elevIndex][floor][btn] = elev.OS_CLEAR
						continue
					}
				}

				// Alle heise enige om clear på denne ordren (btn, floor)
				if isClearedByAll(OC_AllOrderTables, elevIndex, floor, btn, OC_AliveList) {
					primaryOT[elevIndex][floor][btn] = elev.OS_NO_ORDER
					// TODO: make this propagate throught the system again so it actually clears immediately. Now it waits for next OrderTableUpdate.
					continue
				}
				// Alle heisene enige om requesy på denne ordren (btn, floor)
				if isRequestedByAll(OC_AllOrderTables, elevIndex, floor, btn, OC_AliveList) {
					if elevio.ButtonType(btn) == elevio.BT_Cab {
						primaryOT[elevIndex][floor][btn] = elev.OS_CONFIRMED
						continue
					}
					bestID := CalculateWhichElevator(elevIndex, floor, btn, primaryOT, OC_AliveList, OC_NumElevs)
					fmt.Printf("BESTID = %d\n", bestID)
					primaryOT[bestID][floor][btn] = elev.OS_CONFIRMED
					continue
					// Hvis ordren flyttes, nullstill den gamle plassen i Primary sin tabell
					// if bestID != elevIndex {
					// 	primaryOT[elevIndex][floor][btn] = elev.OS_REQUESTED
					// }
				}
				if packet.Id == elevIndex {
					primaryOT[elevIndex][floor][btn] = ElevatorIsOwner(primaryStatus, rcvStatus)
					continue
				}
				// if primaryStatus == elev.OS_NO_ORDER && rcvStatus == elev.OS_REQUESTED {
				// 	primaryOT[elevIndex][floor][btn] = elev.OS_REQUESTED
				// 	// if elevio.ButtonType(btn) != elevio.BT_Cab {
				// 	// 	for e := 0; e < elev.N_MAX_ELEVS; e++ {
				// 	// 		if OC_AliveList[e].Role != elev.ER_Dead {
				// 	// 			primaryOT[e][floor][btn] = elev.OS_REQUESTED
				// 	// 		}
				// 	// 	}
				// 	// 	continue
				// 	// }

				// }

				primaryOT[elevIndex][floor][btn] = CalculateNewOrderStatus(elevIndex, rcvStatus, primaryStatus, packet.Id, OC_PhysicalInfo.Id)
			}
		}
	}

	return primaryOT
}

func ElevatorIsOwner(primaryStatus elev.OrderStatus, rcvStatus elev.OrderStatus) elev.OrderStatus {
	if primaryStatus == elev.OS_CONFIRMED && rcvStatus == elev.OS_CLEAR {
		return elev.OS_CLEAR
	}
	// Otherwise keep primary state unchanged
	return primaryStatus
}

func isReassignable(buttonType elevio.ButtonType, rcvStatus elev.OrderStatus, primaryStatus elev.OrderStatus) bool {
	isHallOrder := buttonType != elevio.BT_Cab
	shouldBeAssigned := (rcvStatus == elev.OS_REQUESTED) && (primaryStatus == elev.OS_NO_ORDER)
	return isHallOrder && shouldBeAssigned
}

func CalculateNewOrderStatus(
	elevIndex int,
	rcvStatus elev.OrderStatus,
	primaryStatus elev.OrderStatus,
	packetID int,
	thisID int) elev.OrderStatus {

	switch primaryStatus {

	case elev.OS_NO_ORDER:
		if rcvStatus == elev.OS_REQUESTED {
			return elev.OS_REQUESTED
		}
		return elev.OS_NO_ORDER

	case elev.OS_REQUESTED:
		// Stay REQUESTED until primary explicitly confirms this elevator via CalculateWhichElevator.
		// If the confirmed elevator dies, this naturally stays REQUESTED and gets reassigned.
		if rcvStatus == elev.OS_CLEAR {
			return elev.OS_NO_ORDER
		}
		return elev.OS_REQUESTED

	case elev.OS_CONFIRMED:
		// Only the owning elevator (elevIndex == packetID) can clear its own confirmed order
		if rcvStatus == elev.OS_CLEAR && packetID == elevIndex {
			return elev.OS_CLEAR
		}
		return elev.OS_CONFIRMED

	case elev.OS_CLEAR:
		// Propagate CLEAR until isClearedByAll(), which then sets NO_ORDER
		return elev.OS_CLEAR

	default:
		fmt.Printf("[CalculateNewOrderStatus] -> default case\n")
		return elev.OS_NO_ORDER
	}
}

func isRequestedByAll(
	AllOrderTables elev.AllOrderTables,
	orderID int,
	floor int,
	btn int,
	AliveList elev.AliveList) bool {

	for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {

		if AliveList[elevIndex].Role == elev.ER_Dead {
			continue
		}

		if AllOrderTables[elevIndex][orderID][floor][btn] != elev.OS_REQUESTED {
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

	for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {

		if AliveList[elevIndex].Role == elev.ER_Dead {
			continue
		}

		thisStatus := AllOrderTables[elevIndex][orderID][floor][btn]
		if thisStatus == elev.OS_REQUESTED || thisStatus == elev.OS_CONFIRMED {
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

	// Large number lol
	minCost := 1000
	bestElevId := elev.INVALID_ELEVID

	for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {

		if AliveList[elevIndex].Role == elev.ER_Dead {
			continue
		}

		currentElev := AliveList[elevIndex]

		LocalOrderTable := orderTableToLOT(OrderTable, currentElev.Id)

		cost := CalculateCost(orderFloor, currentElev, LocalOrderTable)

		if cost < minCost {
			minCost = cost
			bestElevId = currentElev.Id
		}
	}

	if bestElevId == elev.INVALID_ELEVID {
		log.Fatalln("CalculateWhichElevator failed! No elevators found in alivelist.")
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
