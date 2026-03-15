package eventloop

import (
	"elevator-project-g7/internal/elev"
	"elevator-project-g7/internal/elevio"
	"elevator-project-g7/internal/movement"
	"elevator-project-g7/internal/network"
	ordercontrol "elevator-project-g7/internal/orderControl"
	rolemanager "elevator-project-g7/internal/roleManager"
)

type eventLoopCh struct {

	// movement
	mvPhysicalInfo    chan elev.ElevatorPhysicalInfo
	mvFloorArrival    chan int
	mvLocalOrderTable chan elev.LocalOrderTable
	mvMotorDir        chan elevio.MotorDirection
	mvMovement        chan elev.ElevatorMovement
	mvClearOrders     chan elev.ClearOrders

	// ordercontrol
	ocInOrderTablePacket  chan elev.OrderTablePacket
	ocPhysicalInfo        chan elev.ElevatorPhysicalInfo
	ocButtonPress         chan elevio.ButtonEvent
	ocClearOrders         chan elev.ClearOrders
	ocOrderTable          chan elev.OrderTable
	ocOutOrderTablePacket chan elev.OrderTablePacket
	ocAliveList           chan elev.AliveList

	// rolemanager
	rmLocalPhysicalInfo   chan elev.ElevatorPhysicalInfo
	rmNetworkPhysicalInfo chan elev.ElevatorPhysicalInfo
	rmRole                chan elev.ElevatorRole
	rmPrimaryID           chan int
	rmAliveList           chan elev.AliveList
	rmResetVersion        chan int

	// tx
	txPhysicalInfo     chan elev.ElevatorPhysicalInfo
	txRole             chan elev.ElevatorRole
	txOrderTablePacket chan elev.OrderTablePacket
	// rx
	rxRole             chan elev.ElevatorRole
	rxPrimaryId        chan int
	rxResetversion     chan int
	rxPhysicalInfo     chan elev.ElevatorPhysicalInfo
	rxOrderTablePacket chan elev.OrderTablePacket
}

func makeChannels() (
	eventLoopCh,
	rolemanager.Inputs,
	rolemanager.Outputs,
	movement.Inputs,
	movement.Outputs,
	ordercontrol.Inputs,
	ordercontrol.Outputs,
	network.TxInputs,
	network.RxInputs,
	network.RxOutputs,
) {

	ch := eventLoopCh{

		mvPhysicalInfo:    make(chan elev.ElevatorPhysicalInfo, 20),
		mvFloorArrival:    make(chan int, 5),
		mvLocalOrderTable: make(chan elev.LocalOrderTable, 20),
		mvMotorDir:        make(chan elevio.MotorDirection, 20),
		mvMovement:        make(chan elev.ElevatorMovement, 20),
		mvClearOrders:     make(chan elev.ClearOrders, 20),

		ocInOrderTablePacket:  make(chan elev.OrderTablePacket, 100),
		ocPhysicalInfo:        make(chan elev.ElevatorPhysicalInfo, 20),
		ocButtonPress:         make(chan elevio.ButtonEvent, 20),
		ocClearOrders:         make(chan elev.ClearOrders, 20),
		ocOrderTable:          make(chan elev.OrderTable, 20),
		ocOutOrderTablePacket: make(chan elev.OrderTablePacket, 100),
		ocAliveList:           make(chan elev.AliveList, 20),

		rmLocalPhysicalInfo:   make(chan elev.ElevatorPhysicalInfo, 20),
		rmNetworkPhysicalInfo: make(chan elev.ElevatorPhysicalInfo, 100),
		rmRole:                make(chan elev.ElevatorRole, 20),
		rmPrimaryID:           make(chan int, 20),
		rmAliveList:           make(chan elev.AliveList, 20),
		rmResetVersion:        make(chan int, 10),

		txPhysicalInfo:     make(chan elev.ElevatorPhysicalInfo, 20),
		txRole:             make(chan elev.ElevatorRole, 5),
		txOrderTablePacket: make(chan elev.OrderTablePacket, 20),

		rxRole:             make(chan elev.ElevatorRole, 5),
		rxPrimaryId:        make(chan int, 5),
		rxResetversion:     make(chan int, 5),
		rxPhysicalInfo:     make(chan elev.ElevatorPhysicalInfo, 20),
		rxOrderTablePacket: make(chan elev.OrderTablePacket, 100),
	}

	rmInputs := rolemanager.Inputs{
		LocalPhysicalInfo:   ch.rmLocalPhysicalInfo,
		NetworkPhysicalInfo: ch.rmNetworkPhysicalInfo,
	}

	rmOutputs := rolemanager.Outputs{
		Role:         ch.rmRole,
		PrimaryID:    ch.rmPrimaryID,
		AliveList:    ch.rmAliveList,
		ResetVersion: ch.rmResetVersion,
	}

	mvInputs := movement.Inputs{
		PhysicalInfo: ch.mvPhysicalInfo,
		FloorArrival: ch.mvFloorArrival,
	}

	mvOutputs := movement.Outputs{
		LocalOrderTable: ch.mvLocalOrderTable,
		MotorDir:        ch.mvMotorDir,
		Movement:        ch.mvMovement,
		ClearOrders:     ch.mvClearOrders,
	}

	ocInputs := ordercontrol.Inputs{
		PhysicalInfo:     ch.ocPhysicalInfo,
		AliveList:        ch.ocAliveList,
		OrderTablePacket: ch.ocInOrderTablePacket,
		ClearOrders:      ch.ocClearOrders,
		ButtonPress:      ch.ocButtonPress,
	}

	ocOutputs := ordercontrol.Outputs{
		OrderTable:       ch.ocOrderTable,
		OrderTablePacket: ch.ocOutOrderTablePacket,
	}

	txInputs := network.TxInputs{
		PhysicalInfo:     ch.txPhysicalInfo,
		Role:             ch.txRole,
		OrderTablePacket: ch.txOrderTablePacket,
	}

	rxInputs := network.RxInputs{
		Role:         ch.rxRole,
		PrimaryID:    ch.rxPrimaryId,
		ResetVersion: ch.rxResetversion,
	}

	rxOutputs := network.RxOutputs{
		PhysicalInfo:     ch.rxPhysicalInfo,
		OrderTablePacket: ch.rxOrderTablePacket,
	}

	return ch, rmInputs, rmOutputs, mvInputs, mvOutputs, ocInputs, ocOutputs, txInputs, rxInputs, rxOutputs
}
