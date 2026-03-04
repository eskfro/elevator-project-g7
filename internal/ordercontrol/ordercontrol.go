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
					//Som backup var den riktig, men primary må sjekke riktig heis sin orderTable. Sjå 🏷️
					break
				}
				OC_PrevOrderTable = packet.OrderTable
				// Break if backup and message not from primary
				if packet.Id != OC_PhysicalInfo.PrimaryId {
					break
				}
				// Oppdater backup sine verdier ihht primary
				OC_OrderTable = packet.OrderTable
				OC_PhysicalInfo.LocalOrderTable = orderTableToLOT(OC_OrderTable, OC_PhysicalInfo.Id)
				ch_fromOC_OrderTable <- OC_OrderTable
				ch_fromOC_LOT <- OC_PhysicalInfo.LocalOrderTable

			case elev.ER_Primary:

				// Ignorer dersom det er ingen endringer
				if OC_AllOrderTables[packet.Id] == packet.OrderTable { // 🏷️
					break
				}
				OC_AllOrderTables[packet.Id] = packet.OrderTable
				OC_OrderTable = CalculateNewPrimaryOrderTable(OC_OrderTable, OC_AliveList, OC_AllOrderTables, OC_PhysicalInfo, OC_NumElevs, packet)
				// The new primary order table is sent to network and backups are updated accordingly
				elev.PrintOrderTableSlice(OC_OrderTable, packet.Id)
				ch_fromOC_OrderTable <- OC_OrderTable
				ch_fromOC_LOT <- orderTableToLOT(OC_OrderTable, OC_PhysicalInfo.Id)

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

	for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {

		// Skip dead elev
		if OC_AliveList[elevIndex].Role == elev.ER_Dead {
			fmt.Printf("dead\n")
			continue
		}

		for floor := 0; floor < elev.N_FLOORS; floor++ {
			for btn := 0; btn < elev.N_BUTTONS; btn++ {

				primaryStatus := primaryOT[elevIndex][floor][btn]
				rcvStatus := rcvOT[elevIndex][floor][btn]

				if isClearedByAll(OC_AllOrderTables, elevIndex, floor, btn, OC_AliveList) {

					primaryOT[elevIndex][floor][btn] = elev.OS_NO_ORDER
					continue

				} else if isRequestedByAll(OC_AllOrderTables, elevIndex, floor, btn, OC_AliveList) {

					if elevio.ButtonType(btn) == elevio.BT_Cab {

						primaryOT[elevIndex][floor][btn] = elev.OS_CONFIRMED

					} else {

						bestID := CalculateWhichElevator(elevIndex, floor, btn, primaryOT, OC_AliveList, OC_NumElevs)
						fmt.Printf("BESTID = %d\n", bestID)

						// Hvis ordren flyttes, nullstill den gamle plassen i Primary sin tabell
						// Tror egentlig den ikke trengs nå fordi vi er inne i RequestedByAll
						if bestID != elevIndex {
							primaryOT[elevIndex][floor][btn] = elev.OS_REQUESTED
						}

						primaryOT[bestID][floor][btn] = elev.OS_CONFIRMED

					}

				} else if elevio.ButtonType(btn) == elevio.BT_Cab && rcvStatus == elev.OS_REQUESTED {

					primaryOT[elevIndex][floor][btn] = elev.OS_CONFIRMED

				} else if elevio.ButtonType(btn) == elevio.BT_Cab && rcvStatus == elev.OS_CLEAR {

					primaryOT[elevIndex][floor][btn] = elev.OS_NO_ORDER

				} else if primaryStatus == elev.OS_NO_ORDER && rcvStatus == elev.OS_REQUESTED {

					primaryOT[elevIndex][floor][btn] = elev.OS_REQUESTED

				} else if primaryStatus == elev.OS_CONFIRMED && rcvStatus == elev.OS_CLEAR {

					primaryOT[elevIndex][floor][btn] = elev.OS_CLEAR

				} else if primaryStatus == elev.OS_CLEAR && rcvStatus == elev.OS_NO_ORDER {

					primaryOT[elevIndex][floor][btn] = elev.OS_NO_ORDER

				} else {

					primaryOT[elevIndex][floor][btn] = CalculateNewOrderStatus(elevIndex, rcvStatus, primaryStatus, packet.Id, OC_PhysicalInfo.Id)

				}

				// Handle the other cases lol

				/*
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
				*/

			}
		}
	}

	return primaryOT
}

func isReassignable(buttonType elevio.ButtonType, rcvStatus elev.OrderStatus, primaryStatus elev.OrderStatus) bool {
	isHallOrder := buttonType != elevio.BT_Cab
	shouldBeAssigned := (rcvStatus == elev.OS_REQUESTED) && (primaryStatus == elev.OS_NO_ORDER)
	return isHallOrder && shouldBeAssigned
}

// Foreløpig ikkje ferdig funksjon
func CalculateNewOrderStatus(
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
		}

	*/

	//============================= GEMINI END ==============================

	// ============================ Marius og Eskil BEGIN ===================
	//			Foreslår å snu om til switch primarystatus på vår kode og

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

}

// 	//============================== Marius og Eskil END ======================

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
