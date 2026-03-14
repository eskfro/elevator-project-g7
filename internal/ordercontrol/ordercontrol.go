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
	updateOC_PhysicalInfo <-chan elev.ElevatorPhysicalInfo,
	updateOC_AliveList <-chan elev.AliveList,
	fromRX_OrderTableP <-chan elev.OrderTablePacket,
	fromMV_ClearOrders <-chan elev.ClearOrders,
	fromIO_BtnPress <-chan elevio.ButtonEvent,
	fromOC_OrderTable chan<- elev.OrderTable,
	updateTX_OTP chan<- elev.OrderTablePacket,
) {

	// Local to OrderControl
	orderTable := initElev.OrderTable
	allOrderTables := initElev.AllOrderTables
	physicalInfo := initElev.PhysicalInfo
	aliveList := initElev.AliveList

	for {

		select {

		case newPhysicalInfo := <-updateOC_PhysicalInfo:
			log.Println("[OrderControl] PhysicalInfo Update")
			physicalInfo = newPhysicalInfo

		// Directly from rolemanager
		case newAliveList := <-updateOC_AliveList:
			log.Println("[OrderControl] AliveList Update")

			// Remove old data from dead elevators
			for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
				wasDead := aliveList[elevId].Role == elev.ER_Dead
				isAlive := newAliveList[elevId].Role != elev.ER_Dead
				if wasDead && isAlive {
					allOrderTables[elevId] = elev.OrderTable{}
				}
			}

			isAliveListChanged := newAliveList != aliveList
			aliveList = newAliveList

			if !isAliveListChanged {
				continue
			}

			var rcvOrderTable elev.OrderTable
			var ordersToReassign elev.LocalOrderTable

			if physicalInfo.Role == elev.ER_Primary {
				rcvOrderTable, ordersToReassign = resolveDeadElevators(orderTable, aliveList)
				rcvOrderTable = reassignHallOrders(rcvOrderTable, physicalInfo.Id, ordersToReassign)
				allOrderTables[physicalInfo.Id] = rcvOrderTable
			} else {
				rcvOrderTable = orderTable
			}

			// Update states and send
			newOrderTable := updateOrderTable(orderTable, rcvOrderTable, physicalInfo.Id, allOrderTables, physicalInfo, aliveList, updateTX_OTP)
			isOrderTableChanged := orderTable != newOrderTable
			orderTable = newOrderTable
			allOrderTables[physicalInfo.Id] = orderTable

			if isOrderTableChanged {
				fromOC_OrderTable <- orderTable
			}

		// ============================================================================== BTN PRESS FROM IO
		case btnPress := <-fromIO_BtnPress:
			elevio.PrintButtonpress(btnPress)
			rcvOrderTable := orderTable
			currentStatus := orderTable[physicalInfo.Id][btnPress.Floor][btnPress.Button]
			isOrderAlreadyActive := currentStatus == elev.OS_REQUESTED || currentStatus == elev.OS_CONFIRMED
			if isOrderAlreadyActive {
				log.Println("[OrderControl] Order Already Active")
				continue
			}
			rcvOrderTable[physicalInfo.Id][btnPress.Floor][btnPress.Button] = elev.OS_REQUESTED
			// Update states and send
			newOrderTable := updateOrderTable(orderTable, rcvOrderTable, physicalInfo.Id, allOrderTables, physicalInfo, aliveList, updateTX_OTP)
			isOrderTableDifferent := orderTable != newOrderTable
			orderTable = newOrderTable
			allOrderTables[physicalInfo.Id] = orderTable

			if isOrderTableDifferent {
				fromOC_OrderTable <- orderTable
			}

		// =========================================================================== CLEAR ORDER FROM MOVEMENT
		case clearOrders := <-fromMV_ClearOrders:
			log.Println("[OrderControl] Clear Order")
			rcvOrderTable := orderTable

			// Update OrderTable according to incoming clear order
			for btn := 0; btn < elev.N_BUTTONS; btn++ {
				isActiveClearOrder := clearOrders[btn].ElevId != elev.INVALID_ELEVATOR_ID
				if isActiveClearOrder {
					clearOrder := clearOrders[btn]
					rcvOrderTable[clearOrder.ElevId][clearOrder.Floor][clearOrder.ButtonType] = elev.OS_CLEAR
				}
			}
			// Update states and send
			newOrderTable := updateOrderTable(orderTable, rcvOrderTable, physicalInfo.Id, allOrderTables, physicalInfo, aliveList, updateTX_OTP)
			isOrderTableDifferent := orderTable != newOrderTable
			orderTable = newOrderTable
			allOrderTables[physicalInfo.Id] = orderTable

			if isOrderTableDifferent {
				fromOC_OrderTable <- orderTable
			}

		// =========================================================================== PACKET FROM RECIEVER
		case packet := <-fromRX_OrderTableP:
			isMsgFromSelf := packet.Id == physicalInfo.Id
			isChanged := allOrderTables[packet.Id] != packet.OrderTable

			if isMsgFromSelf || !isChanged {
				continue
			}
			// AOT Update
			allOrderTables[packet.Id] = packet.OrderTable
			// Update states and send
			newOrderTable := updateOrderTable(orderTable, packet.OrderTable, packet.Id, allOrderTables, physicalInfo, aliveList, updateTX_OTP)
			isOrderTableDifferent := orderTable != newOrderTable
			orderTable = newOrderTable
			allOrderTables[physicalInfo.Id] = orderTable

			if isOrderTableDifferent {
				fromOC_OrderTable <- orderTable
			}
		}
	}
}

// Her oppdaterer man OrderTable. Det er tre måter OrderTable oppdateres:
// 1. btnPress fra IO
// 2. clearOrder fra Movement
// 3. oppdatering fra primary gjennom nettverket
func updateOrderTable(
	OrderTable elev.OrderTable,
	rcvOrderTable elev.OrderTable,
	rcvId int,
	allOrderTables elev.AllOrderTables,
	physicalInfo elev.ElevatorPhysicalInfo,
	aliveList elev.AliveList,
	updateTX_OTP chan<- elev.OrderTablePacket,
) elev.OrderTable {

	prevOrderTable := OrderTable
	isMsgFromSelf := rcvId == physicalInfo.Id
	isMsgFromPrimary := rcvId == physicalInfo.PrimaryId

	switch physicalInfo.Role {

	case elev.ER_Backup:
		// Backup stupid af 💀

		// New Clear Order or BtnPress
		if isMsgFromSelf || isMsgFromPrimary {
			updateTX_OTP <- elev.OrderTablePacket{Id: physicalInfo.Id, OrderTable: rcvOrderTable}
			return rcvOrderTable
		}

	case elev.ER_Primary:
		primaryOT := OrderTable

		// ClearOrder or BtnPress from Self
		if isMsgFromPrimary {
			primaryOT = rcvOrderTable

			// Resolve dead elevators
			primaryOT, ordersToReassign := resolveDeadElevators(primaryOT, aliveList)
			primaryOT = reassignHallOrders(primaryOT, physicalInfo.Id, ordersToReassign) // Eskil 13.03

			// OrderStatus transitions
			primaryOT = handlePrimaryStatusTransitions(primaryOT, primaryOT, aliveList)
			allOrderTables[physicalInfo.Id] = primaryOT

			primaryOT = assignAndClearOrders(primaryOT, aliveList, allOrderTables, physicalInfo.Id)
			isOrderTableChanged := primaryOT != prevOrderTable

			if isOrderTableChanged {
				// Send to network
				updateTX_OTP <- elev.OrderTablePacket{Id: physicalInfo.Id, OrderTable: primaryOT}
			}

			return primaryOT

			//isMsgFromBackup && isPrimary
		} else {

			// Resolve dead elevators
			primaryOT, ordersToReassign := resolveDeadElevators(primaryOT, aliveList)
			primaryOT = reassignHallOrders(primaryOT, physicalInfo.Id, ordersToReassign) // Eskil 13.03

			// OrderStatus transitions
			primaryOT = handlePrimaryStatusTransitions(primaryOT, rcvOrderTable, aliveList)

			primaryOT = assignAndClearOrders(primaryOT, aliveList, allOrderTables, physicalInfo.Id)
			isOrderTableChanged := primaryOT != prevOrderTable

			if isOrderTableChanged {
				// Send to network
				updateTX_OTP <- elev.OrderTablePacket{Id: physicalInfo.Id, OrderTable: primaryOT}
			}

			return primaryOT
		}
	}

	log.Println("[handleOrderTable] Bottom Return Case")
	return OrderTable
}

func assignAndClearOrders(
	primaryOT elev.OrderTable,
	aliveList elev.AliveList,
	allOrderTables elev.AllOrderTables,
	thisID int,
) elev.OrderTable {

	for floor := 0; floor < elev.N_FLOORS; floor++ {
		for btn := 0; btn < elev.N_BUTTONS; btn++ {
			isCab := elevio.ButtonType(btn) == elevio.BT_Cab

			if isCab {
				for elevId := 0; elevId < elev.N_MAX_ELEVS; elevId++ {
					isDeadElev := aliveList[elevId].Role == elev.ER_Dead
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
				allOrderTables[thisID] = primaryOT
				continue

				//isHall
			} else {
				for orderElevId := 0; orderElevId < elev.N_MAX_ELEVS; orderElevId++ {
					isDeadElev := aliveList[orderElevId].Role == elev.ER_Dead
					if isDeadElev {
						continue
					}

					if isRequestedByAll(allOrderTables, aliveList, floor, btn, orderElevId) {

						if isAlreadyConfirmed(aliveList, primaryOT, floor, btn) {
							continue
						}

						bestElevId := calculateBestElevator(floor, primaryOT, aliveList)
						log.Printf("[OrderControl] bestId = %d\n", bestElevId)

						primaryOT[bestElevId][floor][btn] = elev.OS_CONFIRMED
						allOrderTables[thisID] = primaryOT
						continue
					}

					if isClearedByAny(allOrderTables, aliveList, floor, btn, orderElevId) {
						// Set the Primary OrderTable
						for elevID := 0; elevID < elev.N_MAX_ELEVS; elevID++ {
							isDeadElev := aliveList[elevID].Role == elev.ER_Dead
							if isDeadElev {
								continue
							}
							primaryOT[elevID][floor][btn] = elev.OS_NO_ORDER
						}
						allOrderTables[thisID] = primaryOT
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
	primaryOT elev.OrderTable,
	aliveList elev.AliveList,
) (elev.OrderTable, elev.LocalOrderTable) {

	var ordersToReassign elev.LocalOrderTable

	for elevID := 0; elevID < elev.N_MAX_ELEVS; elevID++ {
		isDeadElev := aliveList[elevID].Role == elev.ER_Dead
		if !isDeadElev {
			continue
		}
		for floor := 0; floor < elev.N_FLOORS; floor++ {
			for btn := 0; btn < elev.N_BUTTONS; btn++ {

				if elevio.ButtonType(btn) == elevio.BT_Cab {
					continue
				}

				primaryStatus := primaryOT[elevID][floor][btn]

				if primaryStatus == elev.OS_CONFIRMED || primaryStatus == elev.OS_REQUESTED {
					primaryOT[elevID][floor][btn] = elev.OS_NO_ORDER
					ordersToReassign[floor][btn] = true
				}
				if primaryStatus == elev.OS_CLEAR {
					primaryOT[elevID][floor][btn] = elev.OS_NO_ORDER
				}
			}
		}
	}
	return primaryOT, ordersToReassign
}

func handlePrimaryStatusTransitions(
	primaryOT elev.OrderTable,
	rcvOT elev.OrderTable,
	AliveList elev.AliveList,
) elev.OrderTable {

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
					primaryOT[elevIndex][floor][btn] = orderStatusTransition(primaryStatus, rcvStatus)
				}
			}
		}
	}
	return primaryOT
}

func orderStatusTransition(
	primaryStatus elev.OrderStatus,
	rcvStatus elev.OrderStatus,
) elev.OrderStatus {

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
		if rcvStatus == elev.OS_CLEAR { // Eskil 13.03 -> Alle kan cleare en confirmed -> Litt usikker men gir mening
			return elev.OS_CLEAR
		}
		return elev.OS_CONFIRMED

	case elev.OS_CLEAR:
		return elev.OS_CLEAR
	}

	// log.Printf("[orderStatusTransition] Base Case Hit\n")
	return primaryStatus
}

func isRequestedByAll(
	allOrderTables elev.AllOrderTables,
	aliveList elev.AliveList,
	floor int,
	btn int,
	orderElevId int,
) bool {

	for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {
		isDeadElev := aliveList[elevIndex].Role == elev.ER_Dead
		if isDeadElev {
			continue
		}
		isRequested := allOrderTables[elevIndex][orderElevId][floor][btn] == elev.OS_REQUESTED
		if !isRequested {
			return false
		}
	}
	return true
}

func isClearedByAny(
	allOrderTables elev.AllOrderTables,
	aliveList elev.AliveList,
	floor int,
	btn int,
	orderElevId int,
) bool {

	for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {
		isDeadElev := aliveList[elevIndex].Role == elev.ER_Dead
		if isDeadElev {
			continue
		}

		if allOrderTables[elevIndex][orderElevId][floor][btn] == elev.OS_CLEAR {
			return true
		}
	}
	return false
}

func isAlreadyConfirmed(AliveList elev.AliveList, OrderTable elev.OrderTable, floor int, btn int) bool {
	for elevID := 0; elevID < elev.N_MAX_ELEVS; elevID++ {
		isDeadElev := AliveList[elevID].Role == elev.ER_Dead
		if isDeadElev {
			continue
		}
		if OrderTable[elevID][floor][btn] == elev.OS_CONFIRMED {
			return true
		}
	}
	return false
}

func calculateBestElevator(
	orderFloor int,
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
		cost := calculateCost(orderFloor, currentElev, OrderTableToLOT(OrderTable, currentElev.Id))
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
func calculateCost(orderFloor int, elevator elev.ElevatorPhysicalInfo, LocalOrderTable elev.LocalOrderTable) int {
	// Cost function penalties
	penaltyFloorDiff := 3
	penaltyNumOrders := 3
	penaltyWrongDir := 10

	numOrders := 0

	floorDiff := int(math.Abs(float64(orderFloor - elevator.Floor)))
	// Count num active orders for elevator
	for floor := 0; floor < elev.N_FLOORS; floor++ {
		for btn := 0; btn < elev.N_BUTTONS; btn++ {
			if LocalOrderTable[floor][btn] {
				numOrders++
			}
		}
	}
	wrongDir := (orderFloor < elevator.Floor && elevator.MotorDir == elevio.MD_Up) || //Elev going up, above the order
		(orderFloor > elevator.Floor && elevator.MotorDir == elevio.MD_Down) || //Elev going down, below the order
		(orderFloor == elevator.Floor && elevator.MotorDir == elevio.MD_Down && requests.RequestBelow(LocalOrderTable, elevator.Floor)) || //Elev just went past the order floor (down)
		(orderFloor == elevator.Floor && elevator.MotorDir == elevio.MD_Up && requests.RequestAbove(LocalOrderTable, elevator.Floor)) //Elev just went past the order floor (up)

	totalCost := penaltyFloorDiff*floorDiff + penaltyNumOrders*numOrders
	if wrongDir {
		totalCost += penaltyWrongDir
	}
	return totalCost

}

func OrderTableToLOT(OrderTable elev.OrderTable, elevId int) elev.LocalOrderTable {
	var localOT elev.LocalOrderTable
	for floor := 0; floor < elev.N_FLOORS; floor++ {
		for btn := 0; btn < elev.N_BUTTONS; btn++ {
			localOT[floor][btn] = OrderTable[elevId][floor][btn] == elev.OS_CONFIRMED
		}
	}
	return localOT
}
