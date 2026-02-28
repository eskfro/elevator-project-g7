
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





# SIMULATOR THINGS
sim0:
	chmod +x scripts/sim.sh
	./scripts/sim.sh 0

sim1:
	chmod +x scripts/sim.sh
	./scripts/sim.sh 1

sim2:
	chmod +x scripts/sim.sh
	./scripts/sim.sh 2

# Change to SimAll on lab
simall:
	chmod +x scripts/simAll.sh
	./scripts/simAll.sh

kill:
	chmod +x scripts/kill.sh
	./scripts/kill.sh
