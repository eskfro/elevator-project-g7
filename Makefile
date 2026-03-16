
TARGET := out

.PHONY: all build run test clean sim

all: build

build: 
	go build -o $(TARGET) main.go

## Build and run
run: build 
	./$(TARGET)

## Run tests with race detection
test:
	go test -v -race ./...

## Remove generated files
clean: 
	go clean
	rm -f $(TARGET)





# =========================== STARTING ELEVS AT HOME
sim0:
	chmod +x scripts/sim.sh
	./scripts/sim.sh 0

sim1:
	chmod +x scripts/sim.sh
	./scripts/sim.sh 1

sim2:
	chmod +x scripts/sim.sh
	./scripts/sim.sh 2

simall:
	chmod +x scripts/simAll.sh
	./scripts/simAll.sh

kill:
	chmod +x scripts/kill.sh
	./scripts/kill.sh

# ========================= STARTING ELEVATORS AT THE LAB
lab0:
	chmod +x scripts/lab.sh
	./scripts/lab.sh 0

lab1:
	chmod +x scripts/lab.sh
	./scripts/lab.sh 1

lab2:
	chmod +x scripts/lab.sh
	./scripts/lab.sh 2
