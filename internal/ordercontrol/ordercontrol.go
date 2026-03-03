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

		case packet := <-ch_RxOrderTableP:

			switch OC_PhysicalInfo.Role {

			case elev.ER_Backup:

				if packet.OrderTable == OC_PrevOrderTable { //	Flytta "if no update" inn i kvar rolle.
					// 												Som backup var den riktig, men primary må sjekke riktig heis sin orderTable. Sjå 🏷️
					break
				}
				OC_PrevOrderTable = packet.OrderTable

				// Break if backup and message not from primary
				if packet.Id != OC_PhysicalInfo.PrimaryId {
					break
				}

				// Oppdater backup sine verdier ihht primary
				OC_OrderTable = packet.OrderTable

				ch_fromOC_OrderTable <- OC_OrderTable
				ch_fromOC_LOT <- orderTableToLOT(OC_OrderTable, OC_PhysicalInfo.Id)

			case elev.ER_Primary:

				if OC_AllOrderTables[packet.Id] == packet.OrderTable { // 🏷️
					break
				}
				OC_AllOrderTables[packet.Id] = packet.OrderTable

				for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {
					for floor := 0; floor < elev.N_FLOORS; floor++ {
						for btn := 0; btn < elev.N_BUTTONS; btn++ {

							primaryStatus := OC_OrderTable[elevIndex][floor][btn]
							rcvStatus := OC_AllOrderTables[packet.Id][elevIndex][floor][btn]

							if isRequestedByAll(OC_AllOrderTables, elevIndex, floor, btn, OC_AliveList) {
								OC_OrderTable[elevIndex][floor][btn] = elev.OS_CONFIRMED
								continue
							}

							if isClearedByAll(OC_AllOrderTables, elevIndex, floor, btn, OC_AliveList) {
								OC_OrderTable[elevIndex][floor][btn] = elev.OS_NO_ORDER
								continue
							}

							if isReassignable(elevio.ButtonType(btn), rcvStatus, primaryStatus) {
								bestID := CalculateWhichElevator(elevIndex, floor, btn, OC_OrderTable, OC_AliveList, OC_NumElevs)

								// Hvis ordren flyttes, nullstill den gamle plassen i Primary sin tabell
								if bestID != elevIndex {
									OC_OrderTable[elevIndex][floor][btn] = elev.OS_NO_ORDER
								}
								OC_OrderTable[bestID][floor][btn] = elev.OS_REQUESTED
								continue
							}

							OC_OrderTable[elevIndex][floor][btn] = CalculateNewStatus(elevIndex, rcvStatus, primaryStatus, packet.Id, OC_PhysicalInfo.Id)

						}
					}
				}
				elev.PrintOrderTableSlice(OC_OrderTable, packet.Id)
				ch_fromOC_OrderTable <- OC_OrderTable
				ch_fromOC_LOT <- orderTableToLOT(OC_OrderTable, OC_PhysicalInfo.Id)

			}
		}
	}
}

func isReassignable(buttonType elevio.ButtonType, rcvStatus elev.OrderStatus, primaryStatus elev.OrderStatus) bool {
	isHallOrder := buttonType != elevio.BT_Cab
	shouldBeAssigned := (rcvStatus == elev.OS_REQUESTED) && (primaryStatus == elev.OS_NO_ORDER)
	return isHallOrder && shouldBeAssigned
}

// Foreløpig ikkje ferdig funksjon
func CalculateNewStatus(
	elevIndex int,
	rcvStatus elev.OrderStatus,
	primaryStatus elev.OrderStatus,
	packetID int,
	thisID int) elev.OrderStatus {

	//======================= GEMINI BEGIN (Funka dårligare enn vår kode) =========================
	/*
		switch primaryStatus {

		case elev.OS_NO_ORDER:
			// Her venter Primary på at noen skal trykke på en knapp
			if rcvStatus == elev.OS_REQUESTED {
				return elev.OS_REQUESTED
			}
			return elev.OS_NO_ORDER

		case elev.OS_REQUESTED:
			// Primary har sett REQUESTED, og venter på at ALLE skal se den.
			// Denne sjekken gjøres i ER_Primary-loopen din vha isRequestedByAll.
			// Hvis ikke alle har sett den ennå, beholder vi REQUESTED.
			return elev.OS_REQUESTED

		case elev.OS_CONFIRMED:
			// Ordren er låst og sendt ut. Nå venter vi på at heisen som
			// eier ordren (elevIndex) skal sette den til CLEAR når den er fremme.
			if rcvStatus == elev.OS_CLEAR && packetID == elevIndex {
				return elev.OS_CLEAR
			}
			return elev.OS_CONFIRMED

		case elev.OS_CLEAR:
			// Primary har sett at ordren er utført. Nå venter vi på at
			// ALLE heiser har satt den til CLEAR (vha isClearedByAll i loopen).
			// Inntil det skjer, beholder vi CLEAR for å "smitte" de andre.
			return elev.OS_CLEAR

		default:
			return elev.OS_NO_ORDER
		}*/
	//============================= GEMINI END ==============================

	// ============================ Marius og Eskil BEGIN ===================
	//				Foreslår å snu om til switch primarystatus på vår kode og
	switch rcvStatus {

	case elev.OS_NO_ORDER:

		if primaryStatus == elev.OS_NO_ORDER {
			return elev.OS_NO_ORDER
		} else if primaryStatus == elev.OS_CLEAR && packetID == thisID {
			return elev.OS_NO_ORDER //Bytta fra clear til no order tirsdags kveld
		} else {
			fmt.Println("Bindestrek 1")
			return elev.OS_NO_ORDER

		}

	case elev.OS_REQUESTED: //La til denne casen igjen tirsdags kveld🔫
		if primaryStatus == elev.OS_NO_ORDER || primaryStatus == elev.OS_REQUESTED {
			return elev.OS_REQUESTED
		} else {
			fmt.Println("Bindestrek 2")
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

	//============================== Marius og Eskil END ======================
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
