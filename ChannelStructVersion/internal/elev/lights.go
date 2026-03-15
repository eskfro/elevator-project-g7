package elev

import "elevator-project-g7/internal/elevio"

// Føkka opp import-greier når eg flytta til elevio
func SetAllLights(localOrderTable LocalOrderTable, orderTable OrderTable, aliveList AliveList) {
	for floor := 0; floor < N_FLOORS; floor++ {
		for btn := 0; btn < N_BUTTONS; btn++ {
			shouldLightUp := false
			btnType := elevio.ButtonType(btn)

			if btnType == elevio.BT_Cab {
				if localOrderTable[floor][btn] {
					shouldLightUp = true
				}
			} else {
				for id, phys := range aliveList {
					if phys.Role == ER_Dead {
						continue
					}
					if orderTable[id][floor][btn] == OS_CONFIRMED {
						shouldLightUp = true
						break
					}
				}
			}

			elevio.SetButtonLamp(btnType, floor, shouldLightUp)
		}
	}
}
