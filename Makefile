
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
	chmod +x scripts/sim0.sh
	./scripts/sim0.sh

sim1:
	chmod +x scripts/sim1.sh
	./scripts/sim1.sh

sim2:
	chmod +x scripts/sim2.sh
	./scripts/sim2.sh


simall:
	chmod +x scripts/simAll.sh
	./scripts/simAll.sh

kill:
	@echo "Closing all gnome-terminal instances..."
	-pkill -f gnome-terminal
