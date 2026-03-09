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
	"sync"
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
			rcvOrderTable := OC_OrderTable
			currentStatus := OC_OrderTable[OC_PhysicalInfo.Id][btnPress.Floor][btnPress.Button]
			isOrderAlreadyActive := currentStatus == elev.OS_REQUESTED || currentStatus == elev.OS_CONFIRMED

			if isOrderAlreadyActive {
				log.Println("[OrderControl] Order Already Active")
				continue
			}

			rcvOrderTable[OC_PhysicalInfo.Id][btnPress.Floor][btnPress.Button] = elev.OS_REQUESTED

			// Update states and send
			OC_OrderTable = updateOrderTable(OC_OrderTable, rcvOrderTable, OC_PhysicalInfo.Id, OC_AllOrderTables, OC_PhysicalInfo, OC_AliveList, ch_updateTX_OTP)
			OC_AllOrderTables[OC_PhysicalInfo.Id] = OC_OrderTable
			OC_PhysicalInfo.LocalOrderTable = orderTableToLOT(OC_OrderTable, OC_PhysicalInfo.Id)
			sendUpdateFromOC(OC_OrderTable, OC_PhysicalInfo.LocalOrderTable, ch_fromOC_OrderTable, ch_fromOC_LOT)

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
			OC_OrderTable = updateOrderTable(OC_OrderTable, rcvOrderTable, OC_PhysicalInfo.Id, OC_AllOrderTables, OC_PhysicalInfo, OC_AliveList, ch_updateTX_OTP)
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

			// Update states and send
			OC_OrderTable = updateOrderTable(OC_OrderTable, packet.OrderTable, packet.Id, OC_AllOrderTables, OC_PhysicalInfo, OC_AliveList, ch_updateTX_OTP)
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
		backupOT := OrderTable

		if isMsgFromPrimary || isMsgFromSelf {
			// Backup stupid af 💀
			backupOT = rcvOrderTable
			ch_updateTX_OTP <- elev.OrderTablePacket{Id: PhysicalInfo.Id, OrderTable: backupOT}

			return backupOT
		}

	case elev.ER_Primary:
		primaryOT := OrderTable
		// clearOrder or btnPress from self
		if isMsgFromPrimary {
			primaryOT = rcvOrderTable
			AllOrderTables[PhysicalInfo.Id] = primaryOT

			// FIRST TRANSITION
			primaryOT = handleStatusTransitions(primaryOT, primaryOT, rcvId, AliveList)
			AllOrderTables[PhysicalInfo.Id] = primaryOT

			// SECOND TRANSITION [Find best elev and clear]
			primaryOT = calculateNewPrimaryOrderTable(primaryOT, AliveList, AllOrderTables, PhysicalInfo)
			isOrderTableChanged := primaryOT != prevOrderTable

			if isOrderTableChanged {
				// Send to network
				ch_updateTX_OTP <- elev.OrderTablePacket{Id: PhysicalInfo.Id, OrderTable: primaryOT}
			}

			return primaryOT

			// Message from a backup over the network
		} else {
			AllOrderTables[rcvId] = rcvOrderTable

			// FIRST TRANSITION
			primaryOT = handleStatusTransitions(primaryOT, rcvOrderTable, rcvId, AliveList)
			AllOrderTables[PhysicalInfo.Id] = primaryOT

			// SECOND TRANSITION [Find best elev and clear]
			primaryOT = calculateNewPrimaryOrderTable(primaryOT, AliveList, AllOrderTables, PhysicalInfo)
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

func handleStatusTransitions(
	OrderTable elev.OrderTable,
	rcvOrderTable elev.OrderTable,
	rcvId int,
	AliveList elev.AliveList,
) elev.OrderTable {

	primaryOT := OrderTable
	rcvOT := rcvOrderTable

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
				isCabCall := elevio.ButtonType(btn) == elevio.BT_Cab

				// Handle cab calls
				if isCabCall {
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

	fmt.Printf("Base case hit! \n")
	return primaryStatus
	// return primaryStatus
}

func calculateNewPrimaryOrderTable(
	OrderTable elev.OrderTable,
	AliveList elev.AliveList,
	AllOrderTables elev.AllOrderTables,
	PhysicalInfo elev.ElevatorPhysicalInfo,
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
					// Clear
					for e := 0; e < elev.N_MAX_ELEVS; e++ {
						primaryOT[e][floor][btn] = elev.OS_NO_ORDER
					}
					// Update primaries table after clearing
					AOT[PhysicalInfo.Id] = primaryOT
					continue
				}

				if isRequestedByAll(AOT, elevIndex, floor, btn, AliveList) {
					// Approve requested cab buttons
					if elevio.ButtonType(btn) == elevio.BT_Cab {
						primaryOT[elevIndex][floor][btn] = elev.OS_CONFIRMED
						AOT[PhysicalInfo.Id] = primaryOT
						continue
					}
					bestID := CalculateWhichElevator(elevIndex, floor, btn, primaryOT, AliveList)
					fmt.Printf("BESTID = %d\n", bestID)
					primaryOT[bestID][floor][btn] = elev.OS_CONFIRMED
					AOT[PhysicalInfo.Id] = primaryOT
					continue

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
		thisStatus := AllOrderTables[elevIndex][orderID][floor][btn]
		if thisStatus != elev.OS_REQUESTED {
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
		isDeadElev := AliveList[elevIndex].Role == elev.ER_Dead
		if isDeadElev {
			continue
		}
		thisStatus := AllOrderTables[elevIndex][orderID][floor][btn]
		if thisStatus != elev.OS_CLEAR {
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

type VersionTracker struct {
	sync.Mutex
	currentVersion uint64
}

func (v *VersionTracker) Increment() uint64 {
	v.Lock()
	defer v.Unlock()
	v.currentVersion++
	return v.currentVersion
}

func (v *VersionTracker) Get() uint64 {
	v.Lock()
	defer v.Unlock()
	return v.currentVersion
}
