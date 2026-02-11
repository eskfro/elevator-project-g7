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
	"elevator-project-g7/internal/rolemanager"
)

type OrderStatus int

const (
	OS_NO_ORDER  OrderStatus = 0
	OS_REQUESTED OrderStatus = 1
	OS_CONFIRMED OrderStatus = 2
	OS_CLEAR     OrderStatus = 3
)

type WorldView struct {
	OrderTable      [elev.N_MAX_ELEVS][elev.N_FLOORS][elev.N_BUTTONS]OrderStatus //Orders for all elevators
	LocalOrderTable [elev.N_FLOORS][elev.N_BUTTONS]bool                          //Local orders assigned by primary
}

func CreateWorldView() WorldView {
	wv := WorldView{}

	//Init OrderTable
	for n_e := 0; n_e < elev.N_MAX_ELEVS; n_e++ {
		for f := 0; f < elev.N_MAX_ELEVS; f++ {
			for b := 0; b < elev.N_MAX_ELEVS; b++ {
				wv.OrderTable[n_e][f][b] = OS_NO_ORDER
			}
		}
	}

	//Init LocalOrderTable
	for f := 0; f < elev.N_MAX_ELEVS; f++ {
		for b := 0; b < elev.N_MAX_ELEVS; b++ {
			wv.LocalOrderTable[f][b] = false
		}
	}

	return wv
}

func CreateAllWorldViews() [elev.N_MAX_ELEVS]WorldView {
	wv := make([]WorldView, elev.N_MAX_ELEVS)

	for i := 0; i < elev.N_MAX_ELEVS; i++ {
		wv[i] = CreateWorldView()
	}
	return wv
}

type Order struct {
	elevatorNumber int
	floor          int
	buttonType     elevio.ButtonType
}

func Start(WorldView *WorldView, AllWorldViews *[elev.N_MAX_ELEVS]WorldView) {

	WorldView := WorldView{}

	//Init OrdeTable to zero
	for n_e := 0; n_e < elev.N_MAX_ELEVS; n_e++ {
		for f := 0; f < elev.N_MAX_ELEVS; f++ {
			for b := 0; b < elev.N_MAX_ELEVS; b++ {
				WorldView.OrderTable[n_e][f][b] = OS_NO_ORDER
			}
		}
	}

	buttonPressCh := make(chan elevio.ButtonEvent)
	clearOrderCh := make(chan elevio.ButtonEvent)
	msgFromPrimaryCh := make(chan [elev.N_MAX_ELEVS][elev.N_FLOORS][elev.N_BUTTONS]OrderStatus)

	go elevio.PollButtons(buttonPressCh)

	go orderControl(&WorldView, buttonPressCh, msgFromPrimaryCh)
}

func isConfirmedByAll(AllWorldViews AllWorldViews, n_e, f, b int) bool {
	for this := 0; this < rolemanager.NumElevs; this++ { //TODO: this er dårlig navn
		if AllWorldViews[this].OrderTable[n_e][f][b] != OS_REQUESTED {
			return false
		}
	}
	return true
}

func orderControl(WorldView *WorldView,
	AllWorldViews *[elev.N_MAX_ELEVS]WorldView, //Primary
	buttonPressCh chan elevio.ButtonEvent,
	msgFromPrimaryCh chan [elev.N_MAX_ELEVS][elev.N_FLOORS][elev.N_BUTTONS]OrderStatus,
	Role *rolemanager.Role) {

	switch Role {

	case ROLE_Backup:
		for {
			select {
			case btnPress := <-ch_buttonPress:
				elevio.PrintButtonpress(btnPress)
				WorldView.OrderTable[rolemanager.Id][btnPress.Floor][btnPress.Button] = OS_REQUESTED

			case OrderTableFromPrimary := <-ch_msgFromPrimary:
				if true { //not acceptance test passed {//TODO: fix if statement
					break //TODO: maybe kill???
				}

				WorldView.OrderTable = OrderTableFromPrimary

				for f := 0; f < elev.N_FLOORS; f++ {
					for b := 0; b < elev.N_BUTTONS; b++ {
						WorldView.LocalOrderTable[f][b] = OrderTableFromPrimary[rolemanager.Id][f][b] == OS_CONFIRMED
					}
				}

			case btnPress := <-ch_clearOrder:
				WorldView.OrderTable[rolemanager.AssignedNumber][btnPress.Floor][btnPress.Button] = OS_CLEAR

			case <-ch_orderBcastTicker:
				//TODO: send heile WorldView til Primary
			}
		}

	case ROLE_Primary:
		for {

			select {

			case <-ch_newOrderRequest:
				//TODO: IdForOrder := Calculate which elevator
				WorldView.OrderTable[IdForOrder][btnPress.Floor][btnPress.Button] = OS_REQUESTED

			case order <- ch_requestConfirmed: //TODO: Fix inputs
				WorldView.LocalOrderTable[order.elevatorNumber][order.floor][order.buttonType] = OS_CONFIRMED

			case rcvWorldView := <-ch_rcvBcast:
				// TODO: Update tables

				for n_e := 0; n_e < rolemanager.NumElevs; n_e++ {
					for f := 0; f < elev.N_FLOORS; f++ {
						for b := 0; b < elev.N_BUTTONS; b++ {

							if AllWorldViews[n_e].OrderTable[n_e][f][b] == OS_CLEAR {
								ch_ //TODO:
							}

							if isConfirmedByAll(AllWorldViews, n_e, f, b) {
								confirmedOrder := Order{
									floor:          f,
									elevatorNumber: n_e,
									buttonType:     b,
								}
								ch_requestConfirmed <- confirmedOrder
							}
						}
					}
				}
			}
		}
	}
}
