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
	ch_fromRX_OrderTableP chan elev.OrderTablePacket,
	ch_fromOC_LOT chan elev.LocalOrderTable,
	ch_fromOC_OrderTable chan elev.OrderTable,
	ch_updateTX_OTP chan elev.OrderTablePacket,
	ch_fromMV_ClearOrders chan elev.ClearOrders,
	ch_fromIO_BtnPress chan elevio.ButtonEvent) {

	// Local to OrderControl
	OC_OrderTable := initElev.OrderTable
	OC_AllOrderTables := initElev.AllOrderTables
	OC_PhysicalInfo := initElev.PhysicalInfo
	OC_AliveList := initElev.AliveList

	for {

		select {

		case newAllOrderTables := <-ch_updateOC_AllOrderTables:
			log.Println("[OrderControl] AllOrderTables Update")
			OC_AllOrderTables = newAllOrderTables

		case newPhysicalInfo := <-ch_updateOC_PhysicalInfo:
			log.Println("[OrderControl] PhysicalInfo Update")
			OC_PhysicalInfo = newPhysicalInfo

		// Directly from rolemanager
		case newAliveList := <-ch_updateOC_AliveList:
			log.Println("[OrderControl] AliveList Update")
			OC_AliveList = newAliveList

		// ============================================================================== BTN PRESS FROM IO
		case btnPress := <-ch_fromIO_BtnPress:
			elevio.PrintButtonpress(btnPress)

			currentStatus := OC_OrderTable[OC_PhysicalInfo.Id][btnPress.Floor][btnPress.Button]
			isOrderActive := currentStatus == elev.OS_REQUESTED || currentStatus == elev.OS_CONFIRMED

			if isOrderActive {
				log.Println("[OrderControl] Order Already Active")
				continue
			}

			OC_OrderTable[OC_PhysicalInfo.Id][btnPress.Floor][btnPress.Button] = elev.OS_REQUESTED
			OC_AllOrderTables[OC_PhysicalInfo.Id] = OC_OrderTable
			packet := elev.OrderTablePacket{Id: OC_PhysicalInfo.Id, OrderTable: OC_OrderTable}

			// Update states and send
			OC_OrderTable = updateOrderTable(OC_OrderTable, OC_AllOrderTables, OC_PhysicalInfo, OC_AliveList, packet, ch_updateTX_OTP)
			OC_AllOrderTables[OC_PhysicalInfo.Id] = OC_OrderTable
			OC_PhysicalInfo.LocalOrderTable = orderTableToLOT(OC_OrderTable, OC_PhysicalInfo.Id)
			sendUpdateFromOC(OC_OrderTable, OC_PhysicalInfo.LocalOrderTable, ch_fromOC_OrderTable, ch_fromOC_LOT)

		// =========================================================================== CLEAR ORDER FROM MV
		case clearOrders := <-ch_fromMV_ClearOrders:
			log.Println("[OrderControl] Clear Order")

			// Update OrderTable according to incoming clear order
			for btn := 0; btn < elev.N_BUTTONS; btn++ {
				isActiveClearOrder := clearOrders[btn].ElevId != elev.INVALID_ELEVATOR_ID
				if isActiveClearOrder {
					clearOrder := clearOrders[btn]
					OC_OrderTable[clearOrder.ElevId][clearOrder.Floor][clearOrder.ButtonType] = elev.OS_CLEAR
				}
			}
			OC_AllOrderTables[OC_PhysicalInfo.Id] = OC_OrderTable
			packet := elev.OrderTablePacket{Id: OC_PhysicalInfo.Id, OrderTable: OC_OrderTable}

			// Update states and send
			OC_OrderTable = updateOrderTable(OC_OrderTable, OC_AllOrderTables, OC_PhysicalInfo, OC_AliveList, packet, ch_updateTX_OTP)
			OC_AllOrderTables[OC_PhysicalInfo.Id] = OC_OrderTable
			OC_PhysicalInfo.LocalOrderTable = orderTableToLOT(OC_OrderTable, OC_PhysicalInfo.Id)
			sendUpdateFromOC(OC_OrderTable, OC_PhysicalInfo.LocalOrderTable, ch_fromOC_OrderTable, ch_fromOC_LOT)

		// =========================================================================== PACKET FROM NETWORK [RX]
		case packet := <-ch_fromRX_OrderTableP:

			// Ignore message from self (This dont make sense for current TCP setup, but will change)
			isMsgFromSelf := packet.Id == OC_PhysicalInfo.Id
			isNoChange := OC_AllOrderTables[packet.Id] == packet.OrderTable

			if isMsgFromSelf || isNoChange {
				continue
			}
			OC_AllOrderTables[packet.Id] = packet.OrderTable

			// Update states and send
			OC_OrderTable = updateOrderTable(OC_OrderTable, OC_AllOrderTables, OC_PhysicalInfo, OC_AliveList, packet, ch_updateTX_OTP)
			OC_AllOrderTables[OC_PhysicalInfo.Id] = OC_OrderTable
			OC_PhysicalInfo.LocalOrderTable = orderTableToLOT(OC_OrderTable, OC_PhysicalInfo.Id)
			sendUpdateFromOC(OC_OrderTable, OC_PhysicalInfo.LocalOrderTable, ch_fromOC_OrderTable, ch_fromOC_LOT)

		}
	}
}

func sendUpdateFromOC(OC_OrderTable elev.OrderTable, OC_LocalOrderTable elev.LocalOrderTable, ch_fromOC_OrderTable chan elev.OrderTable, ch_fromOC_LOT chan elev.LocalOrderTable) {
	ch_fromOC_OrderTable <- OC_OrderTable
	ch_fromOC_LOT <- OC_LocalOrderTable
}

// Her oppdaterer man OrderTable.
// Her er de tre måtene OrderTable oppdateres
// 1. btnPress fra IO
// 2. clearOrder fra Movement
// 3. oppdatering fra primary gjennom nettverket

// Hvis packet.Id != din ID vet du at OrderTable kommer fra nettverket
// Hvis packet.Id == primaryId så oppdaterer du din OrderTable
func updateOrderTable(
	OrderTable elev.OrderTable,
	AllOrderTables elev.AllOrderTables,
	PhysicalInfo elev.ElevatorPhysicalInfo,
	AliveList elev.AliveList,
	packet elev.OrderTablePacket,
	ch_updateTX_OTP chan elev.OrderTablePacket,
) elev.OrderTable {

	isMsgFromSelf := packet.Id == PhysicalInfo.Id
	isMsgFromPrimary := packet.Id == PhysicalInfo.PrimaryId
	prevOrderTable := OrderTable

	switch PhysicalInfo.Role {

	case elev.ER_Backup:

		if isMsgFromPrimary || isMsgFromSelf {
			// Backup is stupid 💀
			OrderTable = packet.OrderTable
			ch_updateTX_OTP <- elev.OrderTablePacket{Id: PhysicalInfo.Id, OrderTable: OrderTable}

			return OrderTable

			// TODO: send OrderTable update kanskje til andre (IDK)
		}

	case elev.ER_Primary:
		// clearOrder or btnPress from self
		if isMsgFromPrimary {
			log.Printf("[OrderControl] isMsgFromPrimary")
			OrderTable = packet.OrderTable
		}
		// First transition
		OrderTable = handleStatusTransitions(OrderTable, packet, AliveList)
		AllOrderTables[PhysicalInfo.Id] = OrderTable
		// Second transition // TODO: Bytt fra packet input til OrderTable og id eller noe
		packet = elev.OrderTablePacket{Id: PhysicalInfo.Id, OrderTable: OrderTable}
		OrderTable = calculateNewPrimaryOrderTable(OrderTable, AliveList, AllOrderTables)

		isOrderTableDifferent := prevOrderTable != OrderTable

		// Send to network and main
		if isOrderTableDifferent {
			ch_updateTX_OTP <- elev.OrderTablePacket{Id: PhysicalInfo.Id, OrderTable: OrderTable}
		}
		return OrderTable
	}

	log.Println("[handleOrderTable] Bottom Return Case")
	return OrderTable
}

func handleStatusTransitions(
	OrderTable elev.OrderTable,
	packet elev.OrderTablePacket,
	AliveList elev.AliveList,
) elev.OrderTable {

	primaryOT := OrderTable
	rcvOT := packet.OrderTable

	for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {
		// Skip if elevator is not on the network (ER_Dead)
		isDeadElev := AliveList[elevIndex].Role == elev.ER_Dead
		if isDeadElev {
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
				} else {
					// State machine logikk for OrderStatus transitions
					primaryOT[elevIndex][floor][btn] = orderStatusTransition(elevIndex, primaryStatus, rcvStatus, packet.Id)
				}
			}
		}
	}
	return primaryOT
}

func orderStatusTransition(
	elevIndex int,
	primaryStatus elev.OrderStatus,
	rcvStatus elev.OrderStatus,
	rcvId int,
) elev.OrderStatus {

	isOwner := rcvId == elevIndex

	switch primaryStatus {

	case elev.OS_NO_ORDER:
		if rcvStatus == elev.OS_REQUESTED {
			return elev.OS_REQUESTED
		}

	case elev.OS_REQUESTED:
		return elev.OS_REQUESTED

	case elev.OS_CONFIRMED:
		if rcvStatus == elev.OS_CLEAR && isOwner {
			return elev.OS_CLEAR
		}

	case elev.OS_CLEAR:
		return elev.OS_CLEAR
	}

	fmt.Printf("Base case hit! \n")

	return primaryStatus
	// return primaryStatus
}

func calculateNewPrimaryOrderTable(
	OrderTable elev.OrderTable,
	AliveList elev.AliveList,
	AllOrderTables elev.AllOrderTables,
) elev.OrderTable {

	primaryOT := OrderTable
	AOT := AllOrderTables

	// Iterate through Primaries OrderTable and the Recieved OrderTable
	for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {
		// Skip if elevator is not on the network (ER_Dead)
		isDeadElev := AliveList[elevIndex].Role == elev.ER_Dead
		if isDeadElev {
			continue
		}
		for floor := 0; floor < elev.N_FLOORS; floor++ {
			for btn := 0; btn < elev.N_BUTTONS; btn++ {
				// Define helpers

				// Alle heise enige om clear på denne ordren (btn, floor)
				if isClearedByAll(AOT, elevIndex, floor, btn, AliveList) {
					for e := 0; e < elev.N_MAX_ELEVS; e++ {
						primaryOT[e][floor][btn] = elev.OS_NO_ORDER
					}
					AOT[elevIndex] = primaryOT
					continue
				}
				// The request trigger will hopefully trigger this
				if isRequestedByAll(AOT, elevIndex, floor, btn, AliveList) {
					// Approve requested cab buttons
					if elevio.ButtonType(btn) == elevio.BT_Cab {
						primaryOT[elevIndex][floor][btn] = elev.OS_CONFIRMED
						AOT[elevIndex] = primaryOT
						continue
					}
					bestID := CalculateWhichElevator(elevIndex, floor, btn, primaryOT, AliveList)
					fmt.Printf("BESTID = %d\n", bestID)
					primaryOT[bestID][floor][btn] = elev.OS_CONFIRMED
					AOT[elevIndex] = primaryOT
					continue
					// Hvis ordren flyttes, nullstill den gamle plassen i Primary sin tabell
					// if bestID != elevIndex {
					// 	primaryOT[elevIndex][floor][btn] = elev.OS_REQUESTED
					// }
				}

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

func isRequestedByAll(
	AllOrderTables elev.AllOrderTables,
	orderID int,
	floor int,
	btn int,
	AliveList elev.AliveList,
) bool {

	for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {
		isDeadElev := AliveList[elevIndex].Role == elev.ER_Dead
		if isDeadElev {
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
	AliveList elev.AliveList,
) bool {

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
) int {

	minCost := math.MaxInt
	bestElevId := math.MaxInt

	for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {
		isDeadElev := AliveList[elevIndex].Role == elev.ER_Dead
		if isDeadElev {
			continue
		}
		currentElev := AliveList[elevIndex]
		cost := CalculateCost(orderFloor, currentElev, orderTableToLOT(OrderTable, currentElev.Id))
		if cost < minCost {
			minCost = cost
			bestElevId = currentElev.Id
		}
	}
	if bestElevId == math.MaxInt {
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
