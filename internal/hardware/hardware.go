package hardware

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"log"
	"time"
)

func Start(
	initElev elev.Elevator,
	fromIO_Obstruction <-chan bool,
	fromIO_Floor <-chan int,
	fromHW_PhysicalInfo chan<- elev.ElevatorPhysicalInfo,
	toMV_FloorArrival chan<- int,
	updateHW_PhysicalInfo <-chan elev.ElevatorPhysicalInfo,
) {
	obstrTicker := time.NewTicker(3000 * time.Millisecond)

	physicalInfo := initElev.PhysicalInfo

	for {
		select {

		case <-obstrTicker.C:
			newObs := elevio.GetObstruction()
			isChangedObs := newObs != physicalInfo.Obstructed
			physicalInfo.Obstructed = newObs
			if isChangedObs {
				fromHW_PhysicalInfo <- physicalInfo
			}

		case physicalInfo = <-updateHW_PhysicalInfo:

		case obst := <-fromIO_Obstruction:
			log.Println("[MAIN] FromIO obs")
			physicalInfo.Obstructed = obst
			fromHW_PhysicalInfo <- physicalInfo

		case floor := <-fromIO_Floor:
			log.Println("[MAIN] FromIO floor")
			physicalInfo.Floor = floor
			toMV_FloorArrival <- floor
			fromHW_PhysicalInfo <- physicalInfo

		}
	}
}
