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
	"time"
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
	var allWorldViews [elev.N_MAX_ELEVS]WorldView
	for i := 0; i < elev.N_MAX_ELEVS; i++ {
		allWorldViews[i] = CreateWorldView()
	}
	return allWorldViews
}

type Order struct {
	elevatorNumber int
	floor          int
	buttonType     elevio.ButtonType
}

type Channels struct {
	buttonPress      chan elevio.ButtonEvent
	clearOrder       chan Order     //Primary
	requestConfirmed chan Order     //Primary
	rcvBcast         chan WorldView //Primary
	newOrderRequest  chan Order     //Primary
	msgFromPrimary   chan [elev.N_MAX_ELEVS][elev.N_FLOORS][elev.N_BUTTONS]OrderStatus
	roleUpdate       chan rolemanager.Role
	orderBcastTicker <-chan time.Time
}

func Start(WorldView *WorldView,
	AllWorldViews *[elev.N_MAX_ELEVS]WorldView,
	Role *rolemanager.Role) {

	//Init OrdeTable to zero
	for n_e := 0; n_e < elev.N_MAX_ELEVS; n_e++ {
		for f := 0; f < elev.N_MAX_ELEVS; f++ {
			for b := 0; b < elev.N_MAX_ELEVS; b++ {
				WorldView.OrderTable[n_e][f][b] = OS_NO_ORDER
			}
		}
	}

	ticker := time.NewTicker(100 * time.Millisecond)

	Ch := Channels{
		buttonPress:      make(chan elevio.ButtonEvent),
		clearOrder:       make(chan Order),     //Primary
		requestConfirmed: make(chan Order),     //Primary
		rcvBcast:         make(chan WorldView), //Primary		 //TODO: WorldView is not a type
		newOrderRequest:  make(chan Order),     //Primary
		msgFromPrimary:   make(chan [elev.N_MAX_ELEVS][elev.N_FLOORS][elev.N_BUTTONS]OrderStatus),
		roleUpdate:       make(chan rolemanager.Role),
		orderBcastTicker: ticker.C,
	}

	go elevio.PollButtons(Ch.buttonPress)
	go rolemanager.PollRoleUpdate(Ch.roleUpdate)

	go orderControl(WorldView, AllWorldViews, Ch)
}

func orderControl(WorldView *WorldView,
	AllWorldViews *[elev.N_MAX_ELEVS]WorldView, //Primary
	Ch Channels) {

	currentRole := rolemanager.ROLE_Backup

	for {
		switch currentRole {
		case rolemanager.ROLE_Backup:
			//Marius: Fjerna for løkker her inne, slik at det går ann å flytte seg mellom Primary og Backup
			select {
			case currentRole = <-Ch.roleUpdate:

			case btnPress := <-Ch.buttonPress:
				elevio.PrintButtonpress(btnPress)
				WorldView.OrderTable[rolemanager.Id][btnPress.Floor][btnPress.Button] = OS_REQUESTED

			case OrderTableFromPrimary := <-Ch.msgFromPrimary:
				if true { //not acceptance test passed {//TODO: fix if statement
					break //TODO: maybe kill???
				}

				WorldView.OrderTable = OrderTableFromPrimary

				for f := 0; f < elev.N_FLOORS; f++ {
					for b := 0; b < elev.N_BUTTONS; b++ {
						WorldView.LocalOrderTable[f][b] = OrderTableFromPrimary[rolemanager.Id][f][b] == OS_CONFIRMED
					}
				}

			case btnPress := <-Ch.clearOrder:
				WorldView.OrderTable[rolemanager.Id][btnPress.floor][btnPress.buttonType] = OS_CLEAR

			case <-Ch.orderBcastTicker:
				//TODO: send heile WorldView til Primary
			}

		case rolemanager.ROLE_Primary:

			select { //TODO: Slik det er no vil goroutinen blokke fram til noken leser (f.eks. Ch.clearOrder <-order)
			//		Ingen vil lese fordi det skjer i samme routine.
			//		Alternativer: - Kjør logikk direkte								⭐⭐⭐
			//					  - Lag funksjoner som kalles direkte				⭐⭐⭐
			//					  - Behold channels men i to separate go-routines	🤨🏎️
			//					  - Behold channels men med buffer					🙅

			case currentRole = <-Ch.roleUpdate:

			case order := <-Ch.newOrderRequest:
				//TODO: IdForOrder := Calculate which elevator
				WorldView.OrderTable[IdForOrder][order.floor][order.buttonType] = OS_REQUESTED

			case order := <-Ch.requestConfirmed:
				WorldView.OrderTable[order.elevatorNumber][order.floor][order.buttonType] = OS_CONFIRMED

			case order := <-Ch.clearOrder:
				//TODO: Acceptance test
				WorldView.OrderTable[order.elevatorNumber][order.floor][order.buttonType] = OS_NO_ORDER

			case rcvWorldView := <-Ch.rcvBcast: //TODO: Sende kun OrderTable og ikkje heile worldview???
				// TODO: Update tables based on changes made

				for n_e := 0; n_e < rolemanager.NumElevs; n_e++ {
					for f := 0; f < elev.N_FLOORS; f++ {
						for b := 0; b < elev.N_BUTTONS; b++ {

							if rcvWorldView.OrderTable[n_e][f][b] == WorldView.OrderTable[n_e][f][b] {
								continue
							}

							order := Order{
								floor:          f,
								elevatorNumber: n_e,
								buttonType:     elevio.ButtonType(b),
							}

							if rcvWorldView.OrderTable[n_e][f][b] == OS_CLEAR {
								Ch.clearOrder <- order
							}

							if isConfirmedByAll(AllWorldViews, order) {
								Ch.requestConfirmed <- order
							}
						}
					}
				}
			}
		}
	}
}

func isConfirmedByAll(AllWorldViews *[elev.N_MAX_ELEVS]WorldView, order Order) bool {
	//Returns true if all backups have the order marked as OS_REQUESTED
	for id := 0; id < rolemanager.NumElevs; id++ {
		if AllWorldViews[id].OrderTable[order.elevatorNumber][order.floor][order.buttonType] != OS_REQUESTED {
			return false
		}
	}
	return true
}
