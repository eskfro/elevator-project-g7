
TARGET := out

.PHONY: all build run test clean

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
