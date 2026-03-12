package ordercontrol

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/requests"
	"log"
	"math"
)

func OrderControl(
	initElev elev.Elevator,
	ch_updateOC_PhysicalInfo chan elev.ElevatorPhysicalInfo,
	ch_updateOC_AliveList chan elev.AliveList,
	ch_fromRX_OrderTableP chan elev.OrderTablePacket,
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

		case newPhysicalInfo := <-ch_updateOC_PhysicalInfo:
			log.Println("[OrderControl] PhysicalInfo Update")
			OC_PhysicalInfo = newPhysicalInfo

		// Directly from rolemanager
		case newAliveList := <-ch_updateOC_AliveList:
			log.Println("[OrderControl] AliveList Update")
			isAliveListChanged := newAliveList != OC_AliveList
			OC_AliveList = newAliveList

			if !isAliveListChanged {
				continue
			}
			var rcvOrderTable elev.OrderTable
			var ordersToReassign elev.LocalOrderTable

			if OC_PhysicalInfo.Role == elev.ER_Primary {
				rcvOrderTable, ordersToReassign = resolveDeadElevators(OC_OrderTable, OC_AliveList)
				rcvOrderTable = reassignHallOrders(rcvOrderTable, OC_PhysicalInfo.Id, ordersToReassign)
				OC_AllOrderTables[OC_PhysicalInfo.Id] = rcvOrderTable
			} else {
				rcvOrderTable = OC_OrderTable
			}

			// Update states and send
			newOrderTable := updateOrderTable(OC_OrderTable, rcvOrderTable, OC_PhysicalInfo.Id, OC_AllOrderTables, OC_PhysicalInfo, OC_AliveList, ch_updateTX_OTP)
			isOrderTableDifferent := OC_OrderTable != newOrderTable
			OC_OrderTable = newOrderTable
			OC_AllOrderTables[OC_PhysicalInfo.Id] = OC_OrderTable

			if isOrderTableDifferent {
				sendUpdateFromOC(OC_OrderTable, ch_fromOC_OrderTable)
			}

		// ============================================================================== BTN PRESS FROM IO
		case btnPress := <-ch_fromIO_BtnPress:
			elevio.PrintButtonpress(btnPress)
			rcvOrderTable := OC_OrderTable
			currentStatus := OC_OrderTable[OC_PhysicalInfo.Id][btnPress.Floor][btnPress.Button]
			isOrderAlreadyActive := currentStatus == elev.OS_REQUESTED || currentStatus == elev.OS_CONFIRMED
			if isOrderAlreadyActive {
				log.Println("[OrderControl] Order Already Active")
				continue
			}
			rcvOrderTable[OC_PhysicalInfo.Id][btnPress.Floor][btnPress.Button] = elev.OS_REQUESTED
			// Update states and send
			newOrderTable := updateOrderTable(OC_OrderTable, rcvOrderTable, OC_PhysicalInfo.Id, OC_AllOrderTables, OC_PhysicalInfo, OC_AliveList, ch_updateTX_OTP)
			isOrderTableDifferent := OC_OrderTable != newOrderTable
			OC_OrderTable = newOrderTable
			OC_AllOrderTables[OC_PhysicalInfo.Id] = OC_OrderTable

			if isOrderTableDifferent {
				sendUpdateFromOC(OC_OrderTable, ch_fromOC_OrderTable)
			}

		// =========================================================================== CLEAR ORDER FROM MV
		case clearOrders := <-ch_fromMV_ClearOrders:
			log.Println("[OrderControl] Clear Order")
			rcvOrderTable := OC_OrderTable

			// Update OrderTable according to incoming clear order
			for btn := 0; btn < elev.N_BUTTONS; btn++ {
				isActiveClearOrder := clearOrders[btn].ElevId != elev.INVALID_ELEVATOR_ID
				if isActiveClearOrder {
					clearOrder := clearOrders[btn]
					rcvOrderTable[clearOrder.ElevId][clearOrder.Floor][clearOrder.ButtonType] = elev.OS_CLEAR
				}
			}
			// Update states and send
			newOrderTable := updateOrderTable(OC_OrderTable, rcvOrderTable, OC_PhysicalInfo.Id, OC_AllOrderTables, OC_PhysicalInfo, OC_AliveList, ch_updateTX_OTP)
			isOrderTableDifferent := OC_OrderTable != newOrderTable
			OC_OrderTable = newOrderTable
			OC_AllOrderTables[OC_PhysicalInfo.Id] = OC_OrderTable

			if isOrderTableDifferent {
				sendUpdateFromOC(OC_OrderTable, ch_fromOC_OrderTable)
			}

		// =========================================================================== PACKET FROM NETWORK [RX]
		case packet := <-ch_fromRX_OrderTableP:
			isMsgFromSelf := packet.Id == OC_PhysicalInfo.Id
			isChange := OC_AllOrderTables[packet.Id] != packet.OrderTable

			if isMsgFromSelf || !isChange {
				continue
			}
			// AOT Update
			OC_AllOrderTables[packet.Id] = packet.OrderTable
			// Update states and send
			newOrderTable := updateOrderTable(OC_OrderTable, packet.OrderTable, packet.Id, OC_AllOrderTables, OC_PhysicalInfo, OC_AliveList, ch_updateTX_OTP)
			isOrderTableDifferent := OC_OrderTable != newOrderTable
			OC_OrderTable = newOrderTable
			OC_AllOrderTables[OC_PhysicalInfo.Id] = OC_OrderTable

			if isOrderTableDifferent {
				sendUpdateFromOC(OC_OrderTable, ch_fromOC_OrderTable)
			}
		}
	}
}

func sendUpdateFromOC(
	OC_OrderTable elev.OrderTable,
	ch_fromOC_OrderTable chan elev.OrderTable,
) {
	ch_fromOC_OrderTable <- OC_OrderTable
}

// Her oppdaterer man OrderTable. Det er tre måter OrderTable oppdateres:
// 1. btnPress fra IO
// 2. clearOrder fra Movement
// 3. oppdatering fra primary gjennom nettverket
func updateOrderTable(
	OrderTable elev.OrderTable,
	rcvOrderTable elev.OrderTable,
	rcvId int,
	AllOrderTables elev.AllOrderTables,
	PhysicalInfo elev.ElevatorPhysicalInfo,
	AliveList elev.AliveList,
	ch_updateTX_OTP chan elev.OrderTablePacket,
) elev.OrderTable {

	prevOrderTable := OrderTable
	isMsgFromSelf := rcvId == PhysicalInfo.Id
	isMsgFromPrimary := rcvId == PhysicalInfo.PrimaryId

	switch PhysicalInfo.Role {

	case elev.ER_Backup:
		// Backup stupid af 💀

		// New Clear Order or BtnPress
		if isMsgFromSelf || isMsgFromPrimary {
			ch_updateTX_OTP <- elev.OrderTablePacket{Id: PhysicalInfo.Id, OrderTable: rcvOrderTable}
			return rcvOrderTable
		}

	case elev.ER_Primary:
		primaryOT := OrderTable

		// ClearOrder or BtnPress from Self
		if isMsgFromPrimary {
			primaryOT = rcvOrderTable

			// FIRST TRANSITION
			primaryOT = handlePrimaryStatusTransitions(primaryOT, primaryOT, rcvId, AliveList)
			AllOrderTables[PhysicalInfo.Id] = primaryOT

			// SECOND TRANSITION
			primaryOT = assignAndClearOrders(primaryOT, AliveList, AllOrderTables, PhysicalInfo)

			isOrderTableChanged := primaryOT != prevOrderTable

			if isOrderTableChanged {
				// Send to network
				ch_updateTX_OTP <- elev.OrderTablePacket{Id: PhysicalInfo.Id, OrderTable: primaryOT}
			}

			return primaryOT

			//isMsgFromBackup && isPrimary
		} else {

			// FIRST TRANSITION
			primaryOT = handlePrimaryStatusTransitions(primaryOT, rcvOrderTable, rcvId, AliveList)

			// SECOND TRANSITION
			primaryOT = assignAndClearOrders(primaryOT, AliveList, AllOrderTables, PhysicalInfo)
			isOrderTableChanged := primaryOT != prevOrderTable

			if isOrderTableChanged {
				// Send to network
				ch_updateTX_OTP <- elev.OrderTablePacket{Id: PhysicalInfo.Id, OrderTable: primaryOT}
			}

			return primaryOT
		}
	}

	log.Println("[handleOrderTable] Bottom Return Case")
	return OrderTable
}

func assignAndClearOrders(
	OrderTable elev.OrderTable,
	AliveList elev.AliveList,
	AllOrderTables elev.AllOrderTables,
	PhysicalInfo elev.ElevatorPhysicalInfo,
) elev.OrderTable {

	thisId := PhysicalInfo.Id
	primaryOT := OrderTable

	// Assign Orders
	for floor := 0; floor < elev.N_FLOORS; floor++ {
		for btn := 0; btn < elev.N_BUTTONS; btn++ {
			isCab := elevio.ButtonType(btn) == elevio.BT_Cab

			if isCab {
				for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
					isDeadElev := AliveList[elevId].Role == elev.ER_Dead
					if isDeadElev {
						continue
					}
					if primaryOT[elevId][floor][btn] == elev.OS_REQUESTED {
						primaryOT[elevId][floor][btn] = elev.OS_CONFIRMED
					}
					if primaryOT[elevId][floor][btn] == elev.OS_CLEAR {
						primaryOT[elevId][floor][btn] = elev.OS_NO_ORDER
					}
				}
				AllOrderTables[thisId] = primaryOT
				continue

				//isHall
			} else {
				for orderElevId := 0; orderElevId < elev.N_MAX_ELEVS; orderElevId++ {
					isDeadElev := AliveList[orderElevId].Role == elev.ER_Dead
					if isDeadElev {
						continue
					}

					if isRequestedByAll(AllOrderTables, AliveList, floor, btn, orderElevId) {

						if existStatusConfirmed(AliveList, primaryOT, floor, btn) {
							continue
						}

						bestElevId := CalculateWhichElevator(floor, btn, primaryOT, AliveList)
						log.Printf("[OrderControl] bestId = %d\n", bestElevId)

						primaryOT[bestElevId][floor][btn] = elev.OS_CONFIRMED
						AllOrderTables[thisId] = primaryOT
						continue
					}

					if isClearedByAny(AllOrderTables, AliveList, floor, btn, orderElevId) {
						// Set the Primary OrderTable
						for e := 0; e < elev.N_MAX_ELEVS; e++ {
							isDeadElev := AliveList[e].Role == elev.ER_Dead
							if isDeadElev {
								continue
							}
							primaryOT[e][floor][btn] = elev.OS_NO_ORDER
						}
						AllOrderTables[thisId] = primaryOT
					}
				}
			}

		}
	}

	return primaryOT
}

func reassignHallOrders(
	primaryOT elev.OrderTable,
	primaryId int,
	ordersToReassign elev.LocalOrderTable,
) elev.OrderTable {

	for floor := 0; floor < elev.N_FLOORS; floor++ {
		for btn := 0; btn < elev.N_BUTTONS; btn++ {
			if ordersToReassign[floor][btn] {
				primaryOT[primaryId][floor][btn] = elev.OS_REQUESTED
			}
		}
	}

	return primaryOT
}

func resolveDeadElevators(
	OrderTable elev.OrderTable,
	AliveList elev.AliveList,
) (elev.OrderTable, elev.LocalOrderTable) {

	primaryOT := OrderTable
	var ordersToReassign elev.LocalOrderTable

	for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
		isDeadElev := AliveList[elevId].Role == elev.ER_Dead
		if !isDeadElev {
			continue
		}
		for floor := 0; floor < elev.N_FLOORS; floor++ {
			for btn := 0; btn < elev.N_BUTTONS; btn++ {

				if elevio.ButtonType(btn) == elevio.BT_Cab {
					continue
				}

				primaryStatus := primaryOT[elevId][floor][btn]

				if primaryStatus == elev.OS_CONFIRMED || primaryStatus == elev.OS_REQUESTED {
					primaryOT[elevId][floor][btn] = elev.OS_NO_ORDER
					ordersToReassign[floor][btn] = true
				}
				if primaryStatus == elev.OS_CLEAR {
					primaryOT[elevId][floor][btn] = elev.OS_NO_ORDER
				}
			}
		}
	}
	return primaryOT, ordersToReassign
}

func handlePrimaryStatusTransitions(
	OrderTable elev.OrderTable,
	rcvOrderTable elev.OrderTable,
	rcvId int,
	AliveList elev.AliveList,
) elev.OrderTable {

	primaryOT := OrderTable
	rcvOT := rcvOrderTable

	for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {
		isDeadElev := AliveList[elevIndex].Role == elev.ER_Dead
		if isDeadElev {
			continue
		}
		for floor := 0; floor < elev.N_FLOORS; floor++ {
			for btn := 0; btn < elev.N_BUTTONS; btn++ {
				// Define helpers
				primaryStatus := primaryOT[elevIndex][floor][btn]
				rcvStatus := rcvOT[elevIndex][floor][btn]
				isCabCall := elevio.ButtonType(btn) == elevio.BT_Cab

				if isCabCall {
					if rcvStatus == elev.OS_REQUESTED {
						primaryOT[elevIndex][floor][btn] = elev.OS_CONFIRMED
						continue
					}
					if rcvStatus == elev.OS_CLEAR {
						primaryOT[elevIndex][floor][btn] = elev.OS_NO_ORDER
						continue
					}
					//isHallCall
				} else {
					// State machine logikk for OrderStatus transitions
					primaryOT[elevIndex][floor][btn] = orderStatusTransition(elevIndex, primaryStatus, rcvStatus, rcvId)
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
		if rcvStatus == elev.OS_CLEAR { //La til denne, idk da
			return elev.OS_CLEAR
		}
		return elev.OS_REQUESTED

	case elev.OS_CONFIRMED:
		if rcvStatus == elev.OS_CLEAR && isOwner {
			return elev.OS_CLEAR
		}

	case elev.OS_CLEAR:
		return elev.OS_CLEAR
	}

	// log.Printf("[orderStatusTransition] Base Case Hit\n")
	return primaryStatus
	// return primaryStatus
}

func isRequestedByAll(
	AllOrderTables elev.AllOrderTables,
	AliveList elev.AliveList,
	floor int,
	btn int,
	orderElevId int,
) bool {

	for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {
		isDeadElev := AliveList[elevIndex].Role == elev.ER_Dead
		if isDeadElev {
			continue
		}
		isRequested := AllOrderTables[elevIndex][orderElevId][floor][btn] == elev.OS_REQUESTED
		if !isRequested {
			return false
		}
	}
	return true
}

func isClearedByAny(
	AllOrderTables elev.AllOrderTables,
	AliveList elev.AliveList,
	floor int,
	btn int,
	orderElevId int,
) bool {

	for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {
		isDeadElev := AliveList[elevIndex].Role == elev.ER_Dead
		if isDeadElev {
			continue
		}
		isClear := AllOrderTables[elevIndex][orderElevId][floor][btn] == elev.OS_CLEAR
		if isClear {
			return true
		}
	}
	return false
}

func existStatusConfirmed(AliveList elev.AliveList, OrderTable elev.OrderTable, floor int, btn int) bool {
	for e := 0; e < elev.N_MAX_ELEVS; e++ {
		isDeadElev := AliveList[e].Role == elev.ER_Dead
		if isDeadElev {
			continue
		}
		if OrderTable[e][floor][btn] == elev.OS_CONFIRMED {
			return true
		}
	}
	return false
}

// Returnerer best heis
// Dette er bare secondary requirement i specen så tenker vi bare lager en enkel algoritme
func CalculateWhichElevator(
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
		cost := CalculateCost(orderFloor, currentElev, OrderTableToLOT(OrderTable, currentElev.Id))
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

func OrderTableToLOT(OrderTable elev.OrderTable, elevId int) elev.LocalOrderTable {
	var LOT elev.LocalOrderTable
	for f := 0; f < elev.N_FLOORS; f++ {
		for b := 0; b < elev.N_BUTTONS; b++ {
			LOT[f][b] = OrderTable[elevId][f][b] == elev.OS_CONFIRMED
		}
	}
	return LOT
}
