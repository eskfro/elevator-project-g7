package ordercontrol

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"log"
	"time"
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
	fromRM_ResetNewElevTimer <-chan struct{},
) {

	startOrderTimer := make(chan elev.Order, 16)
	stopOrderTimer := make(chan elev.Order, 16)
	orderToReassign := make(chan elev.Order, 16)

	newElevTime := time.Now()
	retriggerOT := time.NewTicker(500 * time.Millisecond)
	defer retriggerOT.Stop()

	orderTable := initElev.OrderTable
	allOrderTables := initElev.AllOrderTables
	physicalInfo := initElev.PhysicalInfo
	aliveList := initElev.AliveList

	go monitorOrderTable(startOrderTimer, stopOrderTimer, orderToReassign)

	for {

		select {

		case <-retriggerOT.C:
			if physicalInfo.Role == elev.ER_Primary {
				newOrderTable := assignAndClearOrders(orderTable, aliveList, allOrderTables, physicalInfo.ID, startOrderTimer, stopOrderTimer)
				if newOrderTable != orderTable {
					orderTable = newOrderTable
					allOrderTables[physicalInfo.ID] = orderTable
					fromOC_OrderTable <- orderTable
					updateTX_OTP <- elev.OrderTablePacket{ID: physicalInfo.ID, OrderTable: orderTable}

				}
			}

		case <-fromRM_ResetNewElevTimer:
			log.Println("[OrderControl] Reset New Elev Timer")
			newElevTime = time.Now()

			fromOC_OrderTable <- orderTable
			updateTX_OTP <- elev.OrderTablePacket{ID: physicalInfo.ID, OrderTable: orderTable}

		case newPhysicalInfo := <-updateOC_PhysicalInfo:
			log.Println("[OrderControl] PhysicalInfo Update")
			physicalInfo = newPhysicalInfo

		// ======================================================================= ORDER TO REASSIGN
		case order := <-orderToReassign:
			var rcvOrderTable elev.OrderTable

			if physicalInfo.Role == elev.ER_Primary {

				isActiveOrder := false
				for elevID := 0; elevID < elev.N_MAX_ELEVS; elevID++ {
					if aliveList[elevID].Role == elev.ER_Dead {
						continue
					}
					primaryStatus := orderTable[elevID][order.Floor][order.ButtonType]
					if primaryStatus == elev.OS_CONFIRMED || primaryStatus == elev.OS_REQUESTED {
						isActiveOrder = true
					}
				}

				if isActiveOrder {
					rcvOrderTable = orderTable

					// Clear the old requests
					for elevID := 0; elevID < elev.N_MAX_ELEVS; elevID++ {
						if aliveList[elevID].Role == elev.ER_Dead {
							continue
						}
						rcvOrderTable[elevID][order.Floor][order.ButtonType] = elev.OS_NO_ORDER
					}
					timeoutID := order.ElevID

					bestID := calculateBestElevator(order.Floor, rcvOrderTable, aliveList, timeoutID)
					rcvOrderTable[bestID][order.Floor][order.ButtonType] = elev.OS_CONFIRMED

					orderTable = rcvOrderTable
					allOrderTables[physicalInfo.ID] = orderTable
					fromOC_OrderTable <- orderTable
					updateTX_OTP <- elev.OrderTablePacket{ID: physicalInfo.ID, OrderTable: orderTable}
				}
			}

		// ====================================================================== ALIVELIST FROM ROLEMANAGER
		case newAliveList := <-updateOC_AliveList:
			log.Println("[OrderControl] AliveList Update")

			// Remove old data from dead elevators
			wasAnyDead := false
			for elevID := 0; elevID < elev.N_MAX_ELEVS; elevID++ {
				wasDead := aliveList[elevID].Role == elev.ER_Dead
				isAlive := newAliveList[elevID].Role != elev.ER_Dead
				if wasDead && isAlive {
					// Reset allOrderTables to default when newElev
					allOrderTables[elevID] = elev.OrderTable{}
					wasAnyDead = true
				}
			}

			isAliveListChanged := newAliveList != aliveList
			aliveList = newAliveList

			if !isAliveListChanged && !wasAnyDead {
				continue
			}

			var rcvOrderTable elev.OrderTable
			var ordersToReassign elev.LocalOrderTable

			if physicalInfo.Role == elev.ER_Primary {
				rcvOrderTable, ordersToReassign = resolveDeadElevators(orderTable, aliveList)
				rcvOrderTable = reassignHallOrders(rcvOrderTable, physicalInfo.ID, ordersToReassign)
				allOrderTables[physicalInfo.ID] = rcvOrderTable

			} else {
				rcvOrderTable = orderTable
			}

			newOrderTable := updateOrderTable(orderTable, rcvOrderTable, physicalInfo.ID, allOrderTables, physicalInfo, aliveList, updateTX_OTP, startOrderTimer, stopOrderTimer, newElevTime)
			orderTable = newOrderTable
			allOrderTables[physicalInfo.ID] = orderTable

			fromOC_OrderTable <- orderTable

		// ============================================================================== BTN PRESS FROM IO
		case btnPress := <-fromIO_BtnPress:
			elevio.PrintButtonpress(btnPress)
			rcvOrderTable := orderTable
			currentStatus := orderTable[physicalInfo.ID][btnPress.Floor][btnPress.Button]

			isOrderAlreadyActive := currentStatus == elev.OS_REQUESTED || currentStatus == elev.OS_CONFIRMED

			if isOrderAlreadyActive {
				log.Println("[OrderControl] Order Already Active")
				continue
			}
			rcvOrderTable[physicalInfo.ID][btnPress.Floor][btnPress.Button] = elev.OS_REQUESTED

			newOrderTable := updateOrderTable(orderTable, rcvOrderTable, physicalInfo.ID, allOrderTables, physicalInfo, aliveList, updateTX_OTP, startOrderTimer, stopOrderTimer, newElevTime)
			orderTable = newOrderTable
			allOrderTables[physicalInfo.ID] = orderTable

			fromOC_OrderTable <- orderTable

		// =========================================================================== CLEAR ORDER FROM MOVEMENT
		case clearOrders := <-fromMV_ClearOrders:
			log.Println("[OrderControl] Clear Order")
			rcvOrderTable := orderTable

			for btn := 0; btn < elev.N_BUTTONS; btn++ {
				isActiveClearOrder := clearOrders[btn].ElevID != elev.INVALID_ELEVATOR_ID
				if isActiveClearOrder {
					clearOrder := clearOrders[btn]
					rcvOrderTable[clearOrder.ElevID][clearOrder.Floor][clearOrder.ButtonType] = elev.OS_CLEAR
				}
			}

			newOrderTable := updateOrderTable(orderTable, rcvOrderTable, physicalInfo.ID, allOrderTables, physicalInfo, aliveList, updateTX_OTP, startOrderTimer, stopOrderTimer, newElevTime)
			isChanged := newOrderTable != orderTable
			orderTable = newOrderTable
			allOrderTables[physicalInfo.ID] = orderTable

			if isChanged {
				fromOC_OrderTable <- orderTable
			}

		// =========================================================================== PACKET FROM NETWORK
		case packet := <-fromRX_OrderTableP:
			isMsgFromSelf := packet.ID == physicalInfo.ID
			isChanged := allOrderTables[packet.ID] != packet.OrderTable

			if isMsgFromSelf {
				continue
			}

			shouldIgnoreGuard := time.Since(newElevTime) < elev.RECONNECT_SYNC_WINDOW+500*time.Millisecond

			if !isChanged && !shouldIgnoreGuard {
				continue
			}

			allOrderTables[packet.ID] = packet.OrderTable

			newOrderTable := updateOrderTable(orderTable, packet.OrderTable, packet.ID, allOrderTables, physicalInfo, aliveList, updateTX_OTP, startOrderTimer, stopOrderTimer, newElevTime)
			orderTable = newOrderTable
			allOrderTables[physicalInfo.ID] = orderTable

			fromOC_OrderTable <- orderTable

		}
	}
}

func updateOrderTable(
	OrderTable elev.OrderTable,
	rcvOrderTable elev.OrderTable,
	rcvID int,
	allOrderTables elev.AllOrderTables,
	physicalInfo elev.ElevatorPhysicalInfo,
	aliveList elev.AliveList,
	updateTX_OTP chan<- elev.OrderTablePacket,
	startOrderTimer chan<- elev.Order,
	stopOrderTimer chan<- elev.Order,
	newElevTime time.Time,
) elev.OrderTable {

	isMsgFromSelf := rcvID == physicalInfo.ID
	isMsgFromPrimary := rcvID == physicalInfo.PrimaryID

	switch physicalInfo.Role {

	case elev.ER_Backup:

		if isMsgFromPrimary {
			isNewElevOnNetwork := time.Since(newElevTime) < elev.RECONNECT_SYNC_WINDOW
			if isNewElevOnNetwork {
				updateTX_OTP <- elev.OrderTablePacket{ID: physicalInfo.ID, OrderTable: OrderTable}
				return OrderTable
			} else {
				backupOT := handleBackupTransitions(OrderTable, rcvOrderTable)
				updateTX_OTP <- elev.OrderTablePacket{ID: physicalInfo.ID, OrderTable: backupOT}
				return backupOT
			}

		}
		// Broadcast the new backup orderTable to get approval by primary elevator
		if isMsgFromSelf {
			updateTX_OTP <- elev.OrderTablePacket{ID: physicalInfo.ID, OrderTable: rcvOrderTable}
			return rcvOrderTable
		}

	case elev.ER_Primary:
		primaryOT := OrderTable

		// ClearOrder or BtnPress from self
		if isMsgFromPrimary {
			primaryOT = rcvOrderTable

			primaryOT, ordersToReassign := resolveDeadElevators(primaryOT, aliveList)
			primaryOT = reassignHallOrders(primaryOT, physicalInfo.ID, ordersToReassign)

			primaryOT = handlePrimaryStatusTransitions(primaryOT, primaryOT, aliveList, newElevTime)
			allOrderTables[physicalInfo.ID] = primaryOT

			primaryOT = assignAndClearOrders(primaryOT, aliveList, allOrderTables, physicalInfo.ID, startOrderTimer, stopOrderTimer)
			updateTX_OTP <- elev.OrderTablePacket{ID: physicalInfo.ID, OrderTable: primaryOT}
			return primaryOT

		} else { // OrderTable from backup over network
			primaryOT, ordersToReassign := resolveDeadElevators(primaryOT, aliveList)
			primaryOT = reassignHallOrders(primaryOT, physicalInfo.ID, ordersToReassign)

			// OrderStatus transitions
			primaryOT = handlePrimaryStatusTransitions(primaryOT, rcvOrderTable, aliveList, newElevTime)

			primaryOT = assignAndClearOrders(primaryOT, aliveList, allOrderTables, physicalInfo.ID, startOrderTimer, stopOrderTimer)
			updateTX_OTP <- elev.OrderTablePacket{ID: physicalInfo.ID, OrderTable: primaryOT}
			return primaryOT
		}
	}

	return OrderTable
}

func assignAndClearOrders(
	primaryOT elev.OrderTable,
	aliveList elev.AliveList,
	allOrderTables elev.AllOrderTables,
	thisID int,
	startOrderTimer chan<- elev.Order,
	stopOrderTimer chan<- elev.Order,
) elev.OrderTable {

	for floor := 0; floor < elev.N_FLOORS; floor++ {
		for btn := 0; btn < elev.N_BUTTONS; btn++ {
			isCab := elevio.ButtonType(btn) == elevio.BT_Cab

			if isCab {
				for elevID := 0; elevID < elev.N_MAX_ELEVS; elevID++ {
					isDeadElev := aliveList[elevID].Role == elev.ER_Dead
					if isDeadElev {
						continue
					}
					if primaryOT[elevID][floor][btn] == elev.OS_REQUESTED {
						primaryOT[elevID][floor][btn] = elev.OS_CONFIRMED
					}
					if primaryOT[elevID][floor][btn] == elev.OS_CLEAR {
						primaryOT[elevID][floor][btn] = elev.OS_NO_ORDER
					}
				}
				allOrderTables[thisID] = primaryOT
				continue

			} else { //isHall
				for orderElevID := 0; orderElevID < elev.N_MAX_ELEVS; orderElevID++ {
					isDeadElev := aliveList[orderElevID].Role == elev.ER_Dead
					if isDeadElev {
						continue
					}

					if isClearedByAny(allOrderTables, aliveList, floor, btn, orderElevID) {
						for elevID := 0; elevID < elev.N_MAX_ELEVS; elevID++ {
							isDeadElev := aliveList[elevID].Role == elev.ER_Dead
							if isDeadElev {
								continue
							}
							primaryOT[elevID][floor][btn] = elev.OS_NO_ORDER
							stopOrderTimer <- elev.Order{ElevID: orderElevID, Floor: floor, ButtonType: elevio.ButtonType(btn)}
						}
						allOrderTables[thisID] = primaryOT
						continue
					}

					if isRequestedByAll(allOrderTables, aliveList, floor, btn, orderElevID) {
						if isAlreadyConfirmed(aliveList, primaryOT, floor, btn) {
							continue
						}
						for elevID := 0; elevID < elev.N_MAX_ELEVS; elevID++ {
							if aliveList[elevID].Role == elev.ER_Dead {
								continue
							}
							primaryOT[elevID][floor][btn] = elev.OS_NO_ORDER
						}

						bestElevId := calculateBestElevator(floor, primaryOT, aliveList, elev.INVALID_ELEVATOR_ID)
						log.Printf("[OrderControl] bestId = %d\n", bestElevId)

						primaryOT[bestElevId][floor][btn] = elev.OS_CONFIRMED
						startOrderTimer <- elev.Order{ElevID: bestElevId, Floor: floor, ButtonType: elevio.ButtonType(btn)}
						allOrderTables[thisID] = primaryOT
						continue
					}

				}
			}

		}
	}

	return primaryOT
}

func reassignHallOrders(
	primaryOT elev.OrderTable,
	primaryID int,
	ordersToReassign elev.LocalOrderTable,
) elev.OrderTable {
	for floor := 0; floor < elev.N_FLOORS; floor++ {
		for btn := 0; btn < elev.N_BUTTONS; btn++ {
			if ordersToReassign[floor][btn] {
				primaryOT[primaryID][floor][btn] = elev.OS_REQUESTED
			}
		}
	}
	return primaryOT
}
func handleBackupTransitions(orderTable elev.OrderTable, rcvOrderTable elev.OrderTable) elev.OrderTable {

	backupOT := orderTable

	for elevID := 0; elevID < elev.N_MAX_ELEVS; elevID++ {
		for floor := 0; floor < elev.N_FLOORS; floor++ {
			for btn := 0; btn < elev.N_BUTTONS; btn++ {
				primaryStatus := rcvOrderTable[elevID][floor][btn]
				thisStatus := backupOT[elevID][floor][btn]

				// Hall orders are always overwritten by primary
				if elevio.ButtonType(btn) != elevio.BT_Cab {
					backupOT[elevID][floor][btn] = primaryStatus
					continue
				}

				// Handle cab orders
				newStatus := thisStatus

				switch thisStatus {

				case elev.OS_NO_ORDER:
					if primaryStatus == elev.OS_CONFIRMED || primaryStatus == elev.OS_REQUESTED {
						newStatus = elev.OS_CONFIRMED
					}

				case elev.OS_REQUESTED:
					if primaryStatus == elev.OS_CONFIRMED {
						newStatus = elev.OS_CONFIRMED
					}

				case elev.OS_CONFIRMED:
					if primaryStatus == elev.OS_CLEAR {
						newStatus = elev.OS_NO_ORDER
					}

				case elev.OS_CLEAR:
					newStatus = thisStatus
				}
				backupOT[elevID][floor][btn] = newStatus
			}
		}
	}
	return backupOT
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
	newElevTime time.Time,
) elev.OrderTable {

	for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {
		isDeadElev := AliveList[elevIndex].Role == elev.ER_Dead
		if isDeadElev {
			continue
		}
		for floor := 0; floor < elev.N_FLOORS; floor++ {
			for btn := 0; btn < elev.N_BUTTONS; btn++ {

				primaryStatus := primaryOT[elevIndex][floor][btn]
				rcvStatus := rcvOT[elevIndex][floor][btn]
				isCab := elevio.ButtonType(btn) == elevio.BT_Cab

				if isCab {
					if rcvStatus == elev.OS_REQUESTED {
						primaryOT[elevIndex][floor][btn] = elev.OS_CONFIRMED
						continue
					}
					if rcvStatus == elev.OS_CLEAR {
						primaryOT[elevIndex][floor][btn] = elev.OS_NO_ORDER
						continue
					}

				} else { //isHallCall
					primaryOT[elevIndex][floor][btn] = orderStatusTransition(primaryStatus, rcvStatus, newElevTime)
				}
			}
		}
	}
	return primaryOT
}

func orderStatusTransition(
	primaryStatus elev.OrderStatus,
	rcvStatus elev.OrderStatus,
	newElevTime time.Time,
) elev.OrderStatus {

	switch primaryStatus {

	case elev.OS_NO_ORDER:
		if rcvStatus == elev.OS_REQUESTED {
			return elev.OS_REQUESTED
		}

		if rcvStatus == elev.OS_CONFIRMED && time.Since(newElevTime) < elev.RECONNECT_SYNC_WINDOW {
			return elev.OS_CONFIRMED
		}

	case elev.OS_REQUESTED:
		if rcvStatus == elev.OS_CLEAR {
			return elev.OS_CLEAR
		}
		return elev.OS_REQUESTED

	case elev.OS_CONFIRMED:
		if rcvStatus == elev.OS_CLEAR {
			return elev.OS_CLEAR
		}
		return elev.OS_CONFIRMED

	case elev.OS_CLEAR:
		return elev.OS_CLEAR
	}

	return primaryStatus
}

func isRequestedByAll(
	allOrderTables elev.AllOrderTables,
	aliveList elev.AliveList,
	floor int,
	btn int,
	orderElevID int,
) bool {

	for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {
		isDeadElev := aliveList[elevIndex].Role == elev.ER_Dead
		if isDeadElev {
			continue
		}
		isRequested := allOrderTables[elevIndex][orderElevID][floor][btn] == elev.OS_REQUESTED
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
	orderElevID int,
) bool {

	for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {
		isDeadElev := aliveList[elevIndex].Role == elev.ER_Dead
		if isDeadElev {
			continue
		}

		if allOrderTables[elevIndex][orderElevID][floor][btn] == elev.OS_CLEAR {
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

func OrderTableToLOT(OrderTable elev.OrderTable, elevId int) elev.LocalOrderTable {
	var localOT elev.LocalOrderTable
	for floor := 0; floor < elev.N_FLOORS; floor++ {
		for btn := 0; btn < elev.N_BUTTONS; btn++ {
			localOT[floor][btn] = OrderTable[elevId][floor][btn] == elev.OS_CONFIRMED
		}
	}
	return localOT
}
func HasOrders(lot elev.LocalOrderTable) bool {
	for f := 0; f < elev.N_FLOORS; f++ {
		for b := 0; b < elev.N_BUTTONS; b++ {
			if lot[f][b] {
				return true
			}
		}
	}
	return false
}
