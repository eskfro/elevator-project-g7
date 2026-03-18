package ordercontrol

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/requests"
	rolemanager "elevator-project-g7/internal/roleManager"
	"log"
	"math"
)

// Penalties used in cost function
const (
	penaltyFloorDiff   = 3
	penaltyNumOrders   = 3
	penaltyWrongDir    = 10
	penaltyObstruction = 100
)

func calculateBestElevator(
	orderFloor int,
	OrderTable elev.OrderTable,
	AliveList elev.AliveList,
	timeoutID int,
) int {

	minCost := math.MaxInt
	bestElevID := math.MaxInt
	isOneElev := rolemanager.CountNumElevs(AliveList) == 1

	for elevIndex := 0; elevIndex < elev.N_MAX_ELEVS; elevIndex++ {
		isDeadElev := AliveList[elevIndex].Role == elev.ER_Dead
		if isDeadElev {
			continue
		}
		if isOneElev {
			return elevIndex
		}
		currentElev := AliveList[elevIndex]
		cost := calculateCost(orderFloor, currentElev, OrderTableToLOT(OrderTable, currentElev.ID))
		if cost < minCost && timeoutID != elevIndex {
			minCost = cost
			bestElevID = currentElev.ID
		}
	}
	if bestElevID == math.MaxInt {
		log.Fatalln("CalculateWhichElevator failed! No elevators found in alivelist.")
	}
	log.Printf("[calculateBestElevator] TimeoutID = %d, BestElevId = %d, Cost = %d\n", timeoutID, bestElevID, minCost)
	return bestElevID

}

func calculateCost(
	orderFloor int,
	elevator elev.ElevatorPhysicalInfo,
	LocalOrderTable elev.LocalOrderTable,
) int {

	numOrders := 0

	floorDiff := int(math.Abs(float64(orderFloor - elevator.Floor)))
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
	if elevator.Obstructed {
		totalCost += penaltyObstruction
	}
	return totalCost

}
