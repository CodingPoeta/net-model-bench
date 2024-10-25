build:
	go build -o bin/bench cmd/main.go

run-server: build
	./bin/bench server --mode=$(mode)
