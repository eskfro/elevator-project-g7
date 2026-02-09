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
	"elevator-project-g7/internal/elevio"
	_ "elevator-project-g7/internal/elevio"
	_ "elevator-project-g7/internal/movement"
	_ "elevator-project-g7/internal/network"
	_ "elevator-project-g7/internal/requests"
	_ "elevator-project-g7/internal/rolemanager"
)

func Start() {
	buttonPressCh := make(chan elevio.ButtonEvent)

	go elevio.PollButtons(buttonPressCh)

	go orderControl(buttonPressCh)
}

func orderControl(buttonPressCh chan elevio.ButtonEvent) {
	for {
		select {
		case btnPress := <-buttonPressCh:
			oc_onButtonPress(btnPress)
		//case Får beskjed fra master om å ta en ordre
		}
	}
}

func oc_onButtonPress(buttonPress elevio.ButtonEvent) {
	//TODO: Skriv noke tull her, blant anna send til master
}
