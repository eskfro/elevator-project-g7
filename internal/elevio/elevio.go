package elevio

import "fmt"

func PrintStopButton(chanStopButton chan bool) {
	counter := 0
	for press := range chanStopButton {
		fmt.Println(fmt.Sprintf("Someone pressed the Stop Button [%d] - (%t)", counter, press))

		counter++
	}
}
