package elev

import (
	"elevator-project-g7/internal/elevio"
	"fmt"
	"strings"
)

func PrintElevatorInit(id int, port_HW int) {
	fmt.Printf("\nelevator starting | id = %d | port.Hardware = %d\n\n", id, port_HW)
}

func PrintElevatorInfo(elevator Elevator, uptime float64) {

	confirmed := "  #  "
	inactive := "  -  "
	uptimeString := fmt.Sprintf("%.1f", uptime)

	localOT := elevator.PhysicalInfo.LocalOrderTable

	fmt.Printf("--------------------------------\n")
	fmt.Printf("ELEVATOR %d ", elevator.PhysicalInfo.Id)
	fmt.Printf(" [ %s ] ", elevator.PhysicalInfo.Role)
	fmt.Printf(" < " + elevator.PhysicalInfo.Ip + " > |")
	fmt.Printf(" t = " + uptimeString + "s |")
	fmt.Printf(" primaryId = %d\n", elevator.PhysicalInfo.PrimaryId)
	fmt.Printf("--------------------------------\n")
	fmt.Printf("STATE = %s\n", elevator.PhysicalInfo.Movement)
	fmt.Printf("--------------------------------\n")

	// Print floor row
	fmt.Printf("Floor     |")
	for floor := 0; floor < N_FLOORS; floor++ {
		fmt.Printf("  %d  ", floor)
	}
	fmt.Printf("|\n")

	// Hall Up
	fmt.Printf("Hall Up	  |")
	for floor := 0; floor < N_FLOORS; floor++ {
		s := inactive
		if localOT[floor][elevio.BT_HallUp] {
			s = confirmed
		}
		fmt.Printf(s)
	}
	fmt.Printf("|\n")

	// Hall Up
	fmt.Printf("Hall Down |")
	for floor := 0; floor < N_FLOORS; floor++ {
		status := inactive
		if localOT[floor][elevio.BT_HallDown] {
			status = confirmed
		}
		fmt.Printf(status)
	}
	fmt.Printf("|\n")

	// Hall Up
	fmt.Printf("Cab       |")
	for floor := 0; floor < N_FLOORS; floor++ {
		status := inactive
		if localOT[floor][elevio.BT_Cab] {
			status = confirmed
		}
		fmt.Printf(status)
	}
	fmt.Printf("|\n")

	fmt.Printf("--------------------------------\n")
	fmt.Printf("ALIVELIST | NumElevs = %d\n", elevator.NumElevs)
	fmt.Printf("--------------------------------\n")

	for elevId := 0; elevId < N_MAX_ELEVS; elevId++ {
		fmt.Printf(" %d ", elevId)
	}
	fmt.Printf("\n--------------------------------\n")
	for elevId := 0; elevId < N_MAX_ELEVS; elevId++ {
		if elevator.AliveList[elevId].Role != ER_Dead {
			fmt.Printf(" 1 ")
		} else {
			fmt.Printf(" 0 ")
		}
	}
	fmt.Printf("\n--------------------------------\n\n\n")

}

func (elevRole ElevatorRole) String() string {
	switch elevRole {
	case ER_Backup:
		return "Backup"
	case ER_Primary:
		return "Primary"
	case ER_Dead:
		return "Dead"
	default:
		return "Unknown"
	}
}

func (movement ElevatorMovement) String() string {
	switch movement {
	case EM_DoorOpen:
		return "DoorOpen"
	case EM_Moving:
		return "Moving"
	case EM_Idle:
		return "Idle"
	default:
		return "Unknown"
	}

}

func (status OrderStatus) String() string {
	switch status {
	case OS_CLEAR:
		return "Clear"
	case OS_NO_ORDER:
		return "No order"
	case OS_REQUESTED:
		return "Requested"
	case OS_CONFIRMED:
		return "Confirmed"
	default:
		return "Unknown"
	}
}

func PrintOrderTableSlice(table OrderTable, elevID int) {
	OT_Slice := table[elevID]
	buttonNames := []string{"Hall Up", "Hall Down", "Cab"}

	fmt.Printf("--- Primary's Order Table page %d\n", elevID)

	// Print header: "Button / Floor" etterfulgt av alle etasjenumrene
	fmt.Printf("%-12s", "Button\\Floor")
	for floor := 0; floor < len(OT_Slice); floor++ {
		fmt.Printf(" | Floor %-2d", floor)
	}
	fmt.Println(" |")
	fmt.Println(strings.Repeat("-", 12+len(OT_Slice)*11))

	// Print én rad for hver knappetype
	for btn := 0; btn < len(buttonNames); btn++ {
		fmt.Printf("%-12s", buttonNames[btn])
		for floor := 0; floor < len(OT_Slice); floor++ {
			status := OT_Slice[floor][btn]
			fmt.Printf(" | %-8s", status)
		}
		fmt.Println(" |")
	}
	fmt.Println(strings.Repeat("-", 12+len(OT_Slice)*11))
}
