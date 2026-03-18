
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

race:
	go run -race main.go 0 16657 11311 10411 1

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


# ========================= PACKETLOSS
ploss25:
	chmod +x packet_loss/packetloss.sh
	sudo bash ./packet_loss/packetloss.sh 25 -i 11311 10411

ploss50:
	chmod +x packet_loss/packetloss.sh
	sudo bash ./packet_loss/packetloss.sh 50 -i 11311 10411

ploss100:
	chmod +x packet_loss/packetloss.sh
	sudo bash ./packet_loss/packetloss.sh 100 -i 11311 10411
