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
	ch_toOC_PrimaryOrderTableP chan elev.OrderTablePacket,
	ch_updateTX_OTP chan elev.OrderTablePacket,
	ch_fromMV_ClearOrder chan elev.Order,
	ch_fromIO_BtnPress chan elevio.ButtonEvent) {

	ch_orderTableUpdate := make(chan elev.OrderTablePacket, 10)

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
			OC_OrderTable[OC_PhysicalInfo.Id][btnPress.Floor][btnPress.Button] = elev.OS_REQUESTED
			ch_orderTableUpdate <- elev.OrderTablePacket{Id: OC_PhysicalInfo.Id, OrderTable: OC_OrderTable}
		// =========================================================================== CLEAR ORDER FROM MV
		case clearOrder := <-ch_fromMV_ClearOrder:
			log.Println("[OrderControl] Clear Order")
			OC_OrderTable[OC_PhysicalInfo.Id][clearOrder.Floor][clearOrder.ButtonType] = elev.OS_CLEAR
			ch_orderTableUpdate <- elev.OrderTablePacket{Id: OC_PhysicalInfo.Id, OrderTable: OC_OrderTable}
		// =========================================================================== PACKET FROM NETWORK [RX]
		case packet := <-ch_fromRX_OrderTableP:

			// Ignore message from self (This dont make sense for current TCP setup, but will change)
			isMsgFromSelf := packet.Id == OC_PhysicalInfo.Id
			isNoChange := OC_AllOrderTables[packet.Id] == packet.OrderTable

			if isMsgFromSelf || isNoChange {
				continue
			}
			ch_orderTableUpdate <- packet

		// Her oppdaterer man OrderTable.
		// Her er de tre måtene OrderTable oppdateres
		// 1. btnPress fra IO
		// 2. clearOrder fra Movement
		// 3. oppdatering fra primary gjennom nettverket

		// Hvis packet.Id != din ID vet du at OrderTable kommer fra nettverket
		// Hvis packet.Id == primaryId så oppdaterer du din OrderTable

		case packet := <-ch_orderTableUpdate:

			isMsgFromSelf := packet.Id == OC_PhysicalInfo.Id
			isMsgFromPrimary := packet.Id == OC_PhysicalInfo.PrimaryId
			prevOrderTable := OC_OrderTable

			// Tror vi kan sette denne her
			// Why not liksom, skader ikke at backup vet denne også
			OC_AllOrderTables[packet.Id] = packet.OrderTable

			switch OC_PhysicalInfo.Role {

			case elev.ER_Backup:

				if isMsgFromPrimary || isMsgFromSelf {
					// Backup is stupid 💀
					OC_OrderTable = packet.OrderTable
					OC_AllOrderTables[OC_PhysicalInfo.Id] = OC_OrderTable

					// Backup only updates its OrderTable on the network
					ch_updateTX_OTP <- elev.OrderTablePacket{Id: OC_PhysicalInfo.Id, OrderTable: OC_OrderTable}
					ch_fromOC_LOT <- orderTableToLOT(OC_OrderTable, OC_PhysicalInfo.Id)
					ch_fromOC_OrderTable <- OC_OrderTable

					// TODO: send OrderTable update kanskje til andre (IDK)
				}

			case elev.ER_Primary:

				if isMsgFromPrimary { // clearOrder or btnPress from self
					log.Printf("[OrderControl] isMsgFromPrimary")
					OC_OrderTable = packet.OrderTable
				}
				// First transition
				OC_OrderTable = handleStatusTransitions(OC_OrderTable, packet, OC_AliveList)
				OC_AllOrderTables[OC_PhysicalInfo.Id] = OC_OrderTable
				// Second transition // TODO: Bytt fra packet input til OrderTable og id eller noe
				packet = elev.OrderTablePacket{Id: OC_PhysicalInfo.Id, OrderTable: OC_OrderTable}
				OC_OrderTable = CalculateNewPrimaryOrderTable(OC_OrderTable, OC_AliveList, OC_AllOrderTables, OC_PhysicalInfo, packet)
				OC_AllOrderTables[OC_PhysicalInfo.Id] = OC_OrderTable

				// Update LOT
				OC_PhysicalInfo.LocalOrderTable = orderTableToLOT(OC_OrderTable, OC_PhysicalInfo.Id)

				isOrderTableDifferent := prevOrderTable != OC_OrderTable

				// Send to network and main
				if isOrderTableDifferent {
					ch_updateTX_OTP <- elev.OrderTablePacket{Id: OC_PhysicalInfo.Id, OrderTable: OC_OrderTable}
					ch_fromOC_LOT <- OC_PhysicalInfo.LocalOrderTable
					ch_fromOC_OrderTable <- OC_OrderTable
				}

			}

		}
	}
}

// GAMMEL KODE FOR ARKIVET PER NÅ
/*
case packet := <-ch_fromRX_OrderTableP:

	// Ignore if we have info
	// Also, we dont want to update OC_AllOrderTables yet I think.
	if OC_AllOrderTables[packet.Id] == packet.OrderTable {
		continue
	}

	fmt.Printf("[OrderControl] OrderTablePacket rcvd from Primary\n")
	switch OC_PhysicalInfo.Role {
	// ============================================================ BACKUP RCV OTP
	case elev.ER_Backup:
		// Ignore dersom ingen endring eller OrderTablePacket ikke fra primary
		if packet.Id != OC_PhysicalInfo.PrimaryId {
			break
		}
		OC_OrderTable = packet.OrderTable
		OC_AllOrderTables[OC_PhysicalInfo.Id] = OC_OrderTable
		OC_PhysicalInfo.LocalOrderTable = orderTableToLOT(OC_OrderTable, OC_PhysicalInfo.Id)

		// Set OrderTable and LocalOrderTable the same as the rcv Primary
		ch_updateTX_OTP <- elev.OrderTablePacket{Id: OC_PhysicalInfo.Id, OrderTable: OC_OrderTable}
		ch_fromOC_OrderTable <- OC_OrderTable
		ch_fromOC_LOT <- OC_PhysicalInfo.LocalOrderTable
	// =========================================================== PRIMARY RCV OTP
	case elev.ER_Primary:
		// First transition
		OC_OrderTable = handle_primary_transition(OC_OrderTable, packet, OC_AliveList)
		OC_AllOrderTables[OC_PhysicalInfo.Id] = OC_OrderTable
		packet = elev.OrderTablePacket{Id: OC_PhysicalInfo.Id, OrderTable: OC_OrderTable}

		// Second transition
		OC_OrderTable = CalculateNewPrimaryOrderTable(OC_OrderTable, OC_AliveList, OC_AllOrderTables, OC_PhysicalInfo, packet)

		// Update data
		OC_AllOrderTables[OC_PhysicalInfo.Id] = OC_OrderTable
		OC_PhysicalInfo.LocalOrderTable = orderTableToLOT(OC_OrderTable, OC_PhysicalInfo.Id)

		// Send data
		ch_updateTX_OTP <- elev.OrderTablePacket{Id: OC_PhysicalInfo.PrimaryId, OrderTable: OC_OrderTable}
		ch_fromOC_OrderTable <- OC_OrderTable
		ch_fromOC_LOT <- OC_PhysicalInfo.LocalOrderTable
		//elev.PrintOrderTableSlice(OC_OrderTable, packet.Id)

	}
*/

func handleStatusTransitions(
	OC_OrderTable elev.OrderTable,
	packet elev.OrderTablePacket,
	OC_AliveList elev.AliveList,
) elev.OrderTable {

	rcvOT := packet.OrderTable
	primaryOT := OC_OrderTable

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
				} else {
					// State machine logikk
					primaryOT[elevIndex][floor][btn] = orderStatusTransition(elevIndex, rcvStatus, primaryStatus, packet.Id)
				}
			}
		}
	}
	return primaryOT
}

func orderStatusTransition(
	elevIndex int,
	rcvStatus elev.OrderStatus,
	primaryStatus elev.OrderStatus,
	packetId int,
) elev.OrderStatus {

	switch primaryStatus {

	case elev.OS_NO_ORDER:
		if rcvStatus == elev.OS_REQUESTED {
			return elev.OS_REQUESTED
		}

	case elev.OS_REQUESTED:
		return elev.OS_REQUESTED

	case elev.OS_CONFIRMED:
		if rcvStatus == elev.OS_CLEAR && packetId == elevIndex {
			return elev.OS_CLEAR
		}

	case elev.OS_CLEAR:
		return elev.OS_CLEAR
	}

	fmt.Printf("Base case hit! \n")

	return primaryStatus
	// return primaryStatus
}

func CalculateNewPrimaryOrderTable(
	OC_OrderTable elev.OrderTable,
	OC_AliveList elev.AliveList,
	OC_AllOrderTables elev.AllOrderTables,
	OC_PhysicalInfo elev.ElevatorPhysicalInfo,
	packet elev.OrderTablePacket,
) elev.OrderTable {

	primaryOT := OC_OrderTable
	AOT := OC_AllOrderTables

	// Iterate through Primaries OrderTable and the Recieved OrderTable
	for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {
		// Skip if elevator is not on the network (ER_Dead)
		if OC_AliveList[elevIndex].Role == elev.ER_Dead {
			continue
		}
		for floor := 0; floor < elev.N_FLOORS; floor++ {
			for btn := 0; btn < elev.N_BUTTONS; btn++ {
				// Define helpers

				// Alle heise enige om clear på denne ordren (btn, floor)
				if isClearedByAll(AOT, elevIndex, floor, btn, OC_AliveList) {
					for e := 0; e < elev.N_MAX_ELEVS; e++ {
						primaryOT[e][floor][btn] = elev.OS_NO_ORDER
					}
					AOT[elevIndex] = primaryOT
					continue
				}
				// The request trigger will hopefully trigger this
				if isRequestedByAll(AOT, elevIndex, floor, btn, OC_AliveList) {
					// Approve requested cab buttons
					if elevio.ButtonType(btn) == elevio.BT_Cab {
						primaryOT[elevIndex][floor][btn] = elev.OS_CONFIRMED
						AOT[elevIndex] = primaryOT
						continue
					}
					bestID := CalculateWhichElevator(elevIndex, floor, btn, primaryOT, OC_AliveList)
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

func ElevatorIsOwner(primaryStatus elev.OrderStatus, rcvStatus elev.OrderStatus) elev.OrderStatus {
	if primaryStatus == elev.OS_NO_ORDER && rcvStatus == elev.OS_REQUESTED {
		return elev.OS_REQUESTED
	}
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
	AliveList elev.AliveList) int {

	// Large number lol
	minCost := 1000
	bestElevId := elev.INVALID_ELEVATOR_ID

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
	if bestElevId == elev.INVALID_ELEVATOR_ID {
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

// Regler som bestemmer hvordan primary kan overskrive backup
func valid_p2b_transition(
	backupOT elev.OrderTable,
	primaryOT elev.OrderTable,
	e int, f int, b int) elev.OrderStatus {

	backup_status := backupOT[e][f][b]
	primary_status := primaryOT[e][f][b]

	if backup_status == elev.OS_REQUESTED &&
		primary_status == elev.OS_NO_ORDER {
		return elev.OS_REQUESTED
	}
	if backup_status == elev.OS_NO_ORDER &&
		primary_status == elev.OS_REQUESTED {
		return elev.OS_REQUESTED
	}
	if backup_status == elev.OS_CLEAR &&
		primary_status == elev.OS_NO_ORDER {
		return elev.OS_NO_ORDER
	}
	return primary_status
}
