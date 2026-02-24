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
	"log"
	"math"
)

func OrderControl(
	ch_Update chan elev.Elevator,
	ch_OTPacket chan elev.OrderTablePacket,
	ch_LOTFromOC chan elev.LocalOrderTable) {

	var elevator elev.Elevator

	for {
		switch elevator.PhysicalInfo.Role { //TODO: Bytte rekkefølge på switch Role og events. Bør heller ha for{ select{ case event: if primary {stuff} else {other stuff} } }
		//		 									Viss ikkje vil vi kunne blokke i backup sin select case etter at vi har blitt primary.

		// ======================= BACKUP OC =============================================
		case elev.ER_Backup:
			select {
			case elevator = <-ch_Update:

			//case btnPress := <-ch_ButtonPress:

			case packet := <-ch_OTPacket:
				if false {
					// TODO: Acceptance test
					break
				}

				newLOT := orderTableToLOT(packet.OrderTable, elevator.PhysicalInfo.Id)

				elevator.PhysicalInfo.LocalOrderTable = newLOT

				ch_LOTFromOC <- elevator.PhysicalInfo.LocalOrderTable
			}
		// ======================= PRIMARY OC =============================================
		case elev.ER_Primary:

			for {

				select {

				case elevator = <-ch_Update:

				case packet := <-ch_OTPacket:

					// Flytta til main, under "From network cases":
					// elevator.AllOrderTables[packet.Id] = packet.OrderTable

					newOrderTable := elevator.AllOrderTables[packet.Id]

					for orderID := 0; orderID < int(elevator.NumElevs); orderID++ {
						for floor := 0; floor < elev.N_FLOORS; floor++ {
							for btn := 0; btn < elev.N_BUTTONS; btn++ {

								primaryStatus := elevator.OrderTable[orderID][floor][btn]
								rcvStatus := newOrderTable[orderID][floor][btn]

								if isReassignable(elevio.ButtonType(btn), rcvStatus, primaryStatus) {
									// TODO: fiks variabelnavn OrderID, kan ikke være samme som indeks variabel // Marius: Jo må vere samme, den skal kunne bli overskrevet dersom ordren isReassignable
									// marius se på denne fordi jeg skjønner ikkje // Skal vere good no😎
									orderID = CalculateWhichElevator(orderID, floor, btn, newOrderTable, elevator.AliveList, int(elevator.NumElevs))

								}

								elevator.OrderTable[orderID][floor][btn] = CalculateNewStatus(orderID, floor, btn, rcvStatus, primaryStatus, packet.Id, elevator.AllOrderTables, elevator.AliveList)

							}
						}

					}

					//Ny kode lagd av eskil ()
					// case packet := <-ch_RcvOrderTablePacket:

					// 	// Oppdater AllOrderTables //Marius: Dette er gjort i main no
					// 	elevator.AllOrderTables[packet.Id] = packet.OrderTable

					// 	for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {

					// 		if elevator.AliveList[elevId].Role == elev.ER_Dead {
					// 			continue // TODO: Ka skjer her då? Role manager greier eller Acceptance test?
					// 		}

					// 		for floor := 0; floor < elev.N_FLOORS; floor++ {
					// 			for btn := 0; btn < elev.N_BUTTONS; btn++ {
					// 				//TODO: Her skal vel implementerast noke
					// 			}
					// 		}
					// 	}

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
	AliveList elev.AliveList) elev.OrderStatus {

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
			if isRequestedByAll(AllOrderTables, orderID, floor, btn, AliveList) {
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

func isRequestedByAll(AllOrderTables elev.AllOrderTables,
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
