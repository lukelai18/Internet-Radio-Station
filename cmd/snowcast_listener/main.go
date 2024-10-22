package main

import (
	"fmt"
	"log"
	"net"
	"os"
)

func main(){
	if len(os.Args) != 2{
		log.Fatal("Usage: ./snowcast_listener <udp_port> not correct")
	}

	// Get the udp port number
	udpPort := os.Args[1]
	
	// Get the udp address
	udpAddr, err := net.ResolveUDPAddr("udp", ":" + udpPort)
	if err != nil {
		log.Fatal("Error resolving udp address: ", err)
	}

	// Create a udp listener
	udpSock, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal("Error creating a new socket: ", err)
	}
	defer udpSock.Close()

	log.Printf("The current udpAddr is: %s\n", udpAddr.String())

	buffer := make([]byte, 1500)

	for {
		size, _, err := udpSock.ReadFromUDP(buffer)
		if err != nil {
			log.Fatal("Failed to get UDP packet: ", err)
		}

		fmt.Printf("The size we read is %s\n", string(buffer[:size]))
	}	
}