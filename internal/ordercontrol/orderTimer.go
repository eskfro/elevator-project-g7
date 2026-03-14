package ordercontrol

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/timer"
	"log"
)

func monitorOrderTable(
	startOrderTimer <-chan elev.Order,
	stopOrderTimer <-chan elev.Order,
	orderToReassign chan<- elev.Order,
) {

	var orderTimers [elev.N_MAX_ELEVS][elev.N_FLOORS][elev.N_BUTTONS]*timer.Timer

	for {
		select {
		case newOrderTimer := <-startOrderTimer:

			elevID := newOrderTimer.ElevId
			floor := newOrderTimer.Floor
			btn := newOrderTimer.ButtonType

			if orderTimers[elevID][floor][btn] == nil {

				t := timer.New(elev.ORDER_TIMEOUT)
				orderTimers[elevID][floor][btn] = t

				go func(floor int, btn elevio.ButtonType, elevID int, orderTimeout chan<- elev.Order, timerC <-chan struct{}) {
					for range timerC {
						log.Printf("[OrderControl] Order timer expired for floor %d, button %d\n", floor, btn)

						orderTimeout <- elev.Order{ElevId: elevID, Floor: floor, ButtonType: btn}
					}
				}(floor, btn, elevID, orderToReassign, t.C)

			}
			orderTimers[elevID][floor][btn].Start()

		case stopTimer := <-stopOrderTimer:

			elevID := stopTimer.ElevId
			floor := stopTimer.Floor
			btn := stopTimer.ButtonType

			if orderTimers[elevID][floor][btn] != nil {
				orderTimers[elevID][floor][btn].Stop()
			}

		}
	}
}
