# Makefile

all: 
	go build  ./cmd/snowcast_server
	go build  ./cmd/snowcast_control
	go build  ./cmd/snowcast_listener

clean:
	rm -f $(wildcard snowcast_*)
