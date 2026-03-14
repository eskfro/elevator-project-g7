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

	var orderTimers [elev.N_FLOORS][elev.N_BUTTONS]*timer.Timer

	for {
		select {
		case newOrderTimer := <-startOrderTimer:

			elevID := newOrderTimer.ElevId
			floor := newOrderTimer.Floor
			btn := newOrderTimer.ButtonType

			if orderTimers[floor][btn] == nil {
				t := timer.New(elev.ORDER_TIMEOUT)
				orderTimers[floor][btn] = t

				go func(floor int, btn elevio.ButtonType, elevID int, orderTimeout chan<- elev.Order, timerC <-chan struct{}) {
					for range timerC {
						log.Printf("[OrderControl] Order timer expired for floor %d, button %d\n", floor, btn)

						orderTimeout <- elev.Order{ElevId: elevID, Floor: floor, ButtonType: btn}
					}
				}(floor, btn, elevID, orderToReassign, t.C)

			}
			orderTimers[floor][btn].Start()

		case stopTimer := <-stopOrderTimer:
			floor := stopTimer.Floor
			btn := stopTimer.ButtonType

			if orderTimers[floor][btn] != nil {
				orderTimers[floor][btn].Stop()
			}

		}
	}
}
