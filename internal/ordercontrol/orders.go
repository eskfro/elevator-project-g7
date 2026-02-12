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

type Channels struct {
	buttonPress      chan elevio.ButtonEvent
	clearOrder       chan elev.Order     //Primary
	requestConfirmed chan elev.Order     //Primary
	rcvBcast         chan elev.WorldView //Primary
	newOrderRequest  chan elev.Order     //Primary
	msgFromPrimary   chan [elev.N_MAX_ELEVS][elev.N_FLOORS][elev.N_BUTTONS]elev.OrderStatus
	roleUpdate       chan rolemanager.ElevatorRole
	orderBcastTicker <-chan time.Time
}

func Start(WorldView *elev.WorldView,
	AllWorldViews *[elev.N_MAX_ELEVS]elev.WorldView,
	Role *rolemanager.ElevatorRole) {

	//Init OrdeTable to zero
	for n_e := 0; n_e < elev.N_MAX_ELEVS; n_e++ {
		for f := 0; f < elev.N_MAX_ELEVS; f++ {
			for b := 0; b < elev.N_MAX_ELEVS; b++ {
				WorldView.OrderTable[n_e][f][b] = elev.OS_NO_ORDER
			}
		}
	}

	ticker := time.NewTicker(100 * time.Millisecond)

	Ch := Channels{
		buttonPress:      make(chan elevio.ButtonEvent),
		clearOrder:       make(chan elev.Order),     //Primary
		requestConfirmed: make(chan elev.Order),     //Primary
		rcvBcast:         make(chan elev.WorldView), //Primary		 //TODO: WorldView is not a type
		newOrderRequest:  make(chan elev.Order),     //Primary
		msgFromPrimary:   make(chan [elev.N_MAX_ELEVS][elev.N_FLOORS][elev.N_BUTTONS]elev.OrderStatus),
		roleUpdate:       make(chan rolemanager.ElevatorRole),
		orderBcastTicker: ticker.C,
	}

	go elevio.PollButtons(Ch.buttonPress)
	go rolemanager.PollRoleUpdate(Ch.roleUpdate) //TODO: input to this function

	go orderControl(WorldView, AllWorldViews, Ch)
}

func orderControl(WorldView *elev.WorldView, AllWorldViews *[elev.N_MAX_ELEVS]elev.WorldView, Ch Channels) {

	currentRole := rolemanager.ER_Backup

	for {
		switch currentRole {
		case rolemanager.ER_Backup:
			//Marius: Fjerna for løkker her inne, slik at det går ann å flytte seg mellom Primary og Backup
			select {
			case currentRole = <-Ch.roleUpdate:

			case btnPress := <-Ch.buttonPress:
				elevio.PrintButtonpress(btnPress)
				WorldView.OrderTable[rolemanager.Id][btnPress.Floor][btnPress.Button] = elev.OS_REQUESTED

			case OrderTableFromPrimary := <-Ch.msgFromPrimary:
				if true { //not acceptance test passed {//TODO: fix if statement
					break //TODO: maybe kill???
				}

				WorldView.OrderTable = OrderTableFromPrimary

				for f := 0; f < elev.N_FLOORS; f++ {
					for b := 0; b < elev.N_BUTTONS; b++ {
						WorldView.LocalOrderTable[f][b] = OrderTableFromPrimary[rolemanager.Id][f][b] == elev.OS_CONFIRMED
					}
				}

			case btnPress := <-Ch.clearOrder:
				WorldView.OrderTable[rolemanager.Id][btnPress.Floor][btnPress.ButtonType] = elev.OS_CLEAR

			case <-Ch.orderBcastTicker:
				//TODO: send heile WorldView til Primary
			}

		case rolemanager.ER_Primary:

			select { //TODO: Slik det er no vil goroutinen blokke fram til noken leser (f.eks. Ch.clearOrder <-order)
			//		Ingen vil lese fordi det skjer i samme routine.
			//		Alternativer: - Kjør logikk direkte								⭐⭐⭐
			//					  - Lag funksjoner som kalles direkte				⭐⭐⭐
			//					  - Behold channels men i to separate go-routines	🤨🏎️
			//					  - Behold channels men med buffer					🙅

			case currentRole = <-Ch.roleUpdate:

			case order := <-Ch.newOrderRequest:
				//TODO: IdForOrder := Calculate which elevator
				WorldView.OrderTable[IdForOrder][order.Floor][order.ButtonType] = elev.OS_REQUESTED

			case order := <-Ch.requestConfirmed:
				WorldView.OrderTable[order.ElevatorNumber][order.Floor][order.ButtonType] = elev.OS_CONFIRMED

			case order := <-Ch.clearOrder:
				//TODO: Acceptance test
				WorldView.OrderTable[order.ElevatorNumber][order.Floor][order.ButtonType] = elev.OS_NO_ORDER

			case rcvWorldView := <-Ch.rcvBcast: //TODO: Sende kun OrderTable og ikkje heile worldview???
				// TODO: Update tables based on changes made

				for n_e := 0; n_e < rolemanager.NumElevs; n_e++ {
					for f := 0; f < elev.N_FLOORS; f++ {
						for b := 0; b < elev.N_BUTTONS; b++ {

							if rcvWorldView.OrderTable[n_e][f][b] == WorldView.OrderTable[n_e][f][b] {
								continue
							}

							order := elev.Order{
								Floor:          f,
								ElevatorNumber: n_e,
								ButtonType:     elevio.ButtonType(b),
							}

							if rcvWorldView.OrderTable[n_e][f][b] == elev.OS_CLEAR {
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

func isConfirmedByAll(AllWorldViews *[elev.N_MAX_ELEVS]elev.WorldView, order elev.Order) bool {
	//Returns true if all backups have the order marked as OS_REQUESTED
	for id := 0; id < rolemanager.NumElevs; id++ {
		if AllWorldViews[id].OrderTable[order.ElevatorNumber][order.Floor][order.ButtonType] != elev.OS_REQUESTED {
			return false
		}
	}
	return true
}
