
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

# Start single simulator setup
simone:
	chmod +x scripts/simOne.sh
	./scripts/simOne.sh

# Start triple simulator setup
simall:
	chmod +x scripts/simAll.sh
	./scripts/simAll.sh

## Remove generated files
clean: 
	go clean
	rm -f $(TARGET)

