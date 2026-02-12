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
	ButtonPress      chan elevio.ButtonEvent
	ClearOrder       chan elev.Order     //Primary
	RequestConfirmed chan elev.Order     //Primary
	RcvBcast         chan elev.WorldView //Primary
	NewOrderRequest  chan elev.Order     //Primary
	MsgFromPrimary   chan [elev.N_MAX_ELEVS][elev.N_FLOORS][elev.N_BUTTONS]elev.OrderStatus
	RoleUpdate       chan elev.ElevatorRole
	OrderBcastTicker <-chan time.Time
}

// EAF: I MOVED THIS TO MAIN
/*
func Start(WorldView *elev.WorldView,
	AllWorldViews *[elev.N_MAX_ELEVS]elev.WorldView,
	Role *elev.ElevatorRole) {

	ticker := time.NewTicker(100 * time.Millisecond)

	Ch := Channels{
		buttonPress:      make(chan elevio.ButtonEvent),
		clearOrder:       make(chan elev.Order),     //Primary
		requestConfirmed: make(chan elev.Order),     //Primary
		rcvBcast:         make(chan elev.WorldView), //Primary		 //TODO: WorldView is not a type
		newOrderRequest:  make(chan elev.Order),     //Primary
		msgFromPrimary:   make(chan [elev.N_MAX_ELEVS][elev.N_FLOORS][elev.N_BUTTONS]elev.OrderStatus),
		roleUpdate:       make(chan elev.ElevatorRole),
		orderBcastTicker: ticker.C,
	}

	go elevio.PollButtons(Ch.buttonPress)
	go rolemanager.PollRoleUpdate(Ch.roleUpdate, *Role)
	go orderControl(WorldView, AllWorldViews, Ch)
}

*/

func OrderControl(WorldView *elev.WorldView,
	AllWorldViews *[elev.N_MAX_ELEVS]elev.WorldView,
	Ch Channels,
	NumElevs *int,
	ElevatorId *int,
	currentRole *elev.ElevatorRole) {

	for {
		switch *currentRole {
		case elev.ER_Backup:
			//Marius: Fjerna for løkker her inne, slik at det går ann å flytte seg mellom Primary og Backup
			select {
			case *currentRole = <-Ch.RoleUpdate:

			case btnPress := <-Ch.ButtonPress:
				elevio.PrintButtonpress(btnPress)
				WorldView.OrderTable[*ElevatorId][btnPress.Floor][btnPress.Button] = elev.OS_REQUESTED

			case OrderTableFromPrimary := <-Ch.MsgFromPrimary:
				if true { //not acceptance test passed {//TODO: fix if statement
					break //TODO: maybe kill???
				}

				WorldView.OrderTable = OrderTableFromPrimary

				for f := 0; f < elev.N_FLOORS; f++ {
					for b := 0; b < elev.N_BUTTONS; b++ {
						WorldView.LocalOrderTable[f][b] = OrderTableFromPrimary[rolemanager.Id][f][b] == elev.OS_CONFIRMED
					}
				}

			case btnPress := <-Ch.ClearOrder:
				WorldView.OrderTable[*ElevatorId][btnPress.Floor][btnPress.ButtonType] = elev.OS_CLEAR

			case <-Ch.OrderBcastTicker:
				//TODO: send heile WorldView til Primary
			}

		case elev.ER_Primary:

			select { //TODO: Slik det er no vil goroutinen blokke fram til noken leser (f.eks. Ch.clearOrder <-order)
			//		Ingen vil lese fordi det skjer i samme routine.
			//		Alternativer: - Kjør logikk direkte								⭐⭐⭐
			//					  - Lag funksjoner som kalles direkte				⭐⭐⭐
			//					  - Behold channels men i to separate go-routines	🤨🏎️
			//					  - Behold channels men med buffer					🙅

			case *currentRole = <-Ch.RoleUpdate:

			case order := <-Ch.NewOrderRequest:
				//TODO: IdForOrder := Calculate which elevator
				WorldView.OrderTable[IdForOrder][order.Floor][order.ButtonType] = elev.OS_REQUESTED

			case order := <-Ch.RequestConfirmed:
				WorldView.OrderTable[order.ElevatorNumber][order.Floor][order.ButtonType] = elev.OS_CONFIRMED

			case order := <-Ch.ClearOrder:
				//TODO: Acceptance test
				WorldView.OrderTable[order.ElevatorNumber][order.Floor][order.ButtonType] = elev.OS_NO_ORDER

			case rcvWorldView := <-Ch.RcvBcast: //TODO: Sende kun OrderTable og ikkje heile worldview???
				// TODO: Update tables based on changes made

				for n_e := 0; n_e < *NumElevs; n_e++ {
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
								Ch.ClearOrder <- order
							}

							if isConfirmedByAll(AllWorldViews, order, NumElevs) {
								Ch.RequestConfirmed <- order
							}
						}
					}
				}
			}
		}
	}
}

func isConfirmedByAll(AllWorldViews *[elev.N_MAX_ELEVS]elev.WorldView, order elev.Order, NumElevs *int) bool {
	//Returns true if all backups have the order marked as OS_REQUESTED
	for id := 0; id < *NumElevs; id++ {
		if AllWorldViews[id].OrderTable[order.ElevatorNumber][order.Floor][order.ButtonType] != elev.OS_REQUESTED {
			return false
		}
	}
	return true
}
