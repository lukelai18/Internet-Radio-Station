package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"snowcast/pkg/protocol"
	"strconv"
	"strings"
	"time"
)

type ServerMessage struct {
	Type    uint8
	Payload interface{}
}

type UserCommand struct {
    Command string
    Args    []string
}

type ClientState struct {
    sentHello       bool
    receivedWelcome bool
    sentSetStation  bool
}

func main(){
	if len(os.Args) != 4 {
		log.Fatal("Usage: ./snowcast_control <server IP> <server port> <listener port> not correct")
	}

	// The limit of stations in the server
	var maxStation uint16
	var clientState ClientState

	// Get the parameters
	server_IP := os.Args[1]
	server_port := os.Args[2]
	listen_portStr := os.Args[3]
	log.Printf("Current listening port is: %s \n", listen_portStr)

	// Convert listen_portStr from string to int
	listen_port, err := strconv.ParseUint(listen_portStr, 10, 16)
	if err != nil {
		log.Fatal("Invalid listen_port")
	}

	sock, err := net.Dial("tcp4", net.JoinHostPort(server_IP, server_port))

	if err != nil {
		log.Fatal(err)
	}
	defer sock.Close()

	writeHelloToServer(sock, listen_port, &clientState)

	// Set up channels for communication
	msgChan := make(chan ServerMessage)
	done := make(chan struct{})
	userInputChan := make(chan string)
	
	// Start listen to server message
	go listenToServer(sock, msgChan, done, &clientState)
	
	// Handle the user input
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for {
			// Simulate the real user input in termial
			fmt.Printf("> ")
			if !scanner.Scan() {
				// Log the error and close the channel
				log.Println("Input scanner closed")
				close(userInputChan)
				return 
			}
			// If the scan works correctly
			line := scanner.Text()
			userInputChan <- line
		}
	}()

	for {
		select {
		case msg := <-msgChan:
			switch  msg.Type {
			// Wecome Message
			case 2:
				welcome := msg.Payload.(*protocol.Welcome)
				fmt.Printf("Welcome to Snowcast! The server has %d stations\n", welcome.NumStations)
				maxStation = welcome.NumStations
			// Announce message
			case 3:
				songName := msg.Payload.(string)
				fmt.Printf("New song announced: %s\n", songName)
			// Invalid Message
			case 4:
				invalidMsg := msg.Payload.(string)
				log.Printf("Invalid command: %s\n", invalidMsg)
			default:
				log.Printf("Unknown message type %d received", msg.Type)
			}
		
		case inputVal, ok := <-userInputChan:
			if !ok {
				log.Println("Failed to receive the input from terminal")
				return
			}
			handleUserInput(sock, inputVal, maxStation, &clientState)
		
		case <- done:
			log.Println("Connection closed by server")
			os.Exit(1)
		}
	}
}

// A function to hanlde the input from user
func handleUserInput(sock net.Conn, input string, maxStation uint16, clientState *ClientState) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	// Quit the client if receive q
	if input == "q" {
		log.Println("Quit the client")
		sock.Close()
		os.Exit(0)
	}

	// Get the input station number
	stationNum, err := strconv.Atoi(input)
	if err != nil {
		log.Printf("Enter a valid number to continue, the current input is: %s\n", input)
		return
	}

	// Check if the input station number is valid
	if stationNum < 0 || stationNum >= int(maxStation) {
		log.Printf("Enter a number in the valid range, current num: %d, the range should in %d\n", 
			stationNum, int(maxStation) - 1)
		return
	}

	// Now we get the correct input station, begin to sent the set station message
	setStationMsg := &protocol.SetStation{
		CommandType: 1,
		StationNumber: uint16(stationNum),
	}

	setStationBytes, err := setStationMsg.Marshal()
	if err != nil {
		log.Printf("Failed to marshal the set station message: %v\n", err)
		return
	}

	_, err = sock.Write(setStationBytes)
	if err != nil {
		log.Printf("Failed to sent set station message to server: %v\n", err)
		return
	}

	clientState.sentSetStation = true
	log.Printf("Succefully sending set station message to server, the station number is: %d\n", stationNum)
}


func writeHelloToServer(sock net.Conn, listen_port uint64, clientState *ClientState) {
	hello:= &protocol.Hello{
		CommandType: 0,
		UdpPort: uint16(listen_port),
	}

	helloBytes, err := hello.Marshal()
	if err != nil {
		log.Panic("Failed to handle hello Marshal: ", err)
	}

	_, err = sock.Write(helloBytes)
	if err != nil {
		log.Panic("Failed to send hello message to server: ", err)
	}

	clientState.sentHello = true
}

func listenToServer(sock net.Conn, msgChan chan<- ServerMessage, done chan<- struct{}, clientState *ClientState) {
	defer close(msgChan)
	var welcomeCount int

	for {
		if clientState.sentHello && !clientState.receivedWelcome {
            // Set a 100ms deadline waiting for Welcome message
            sock.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
        } else {
            // No read deadline
            sock.SetReadDeadline(time.Time{})
        }

		replyType := make([]byte, 1)
		_, err := sock.Read(replyType)
		if err != nil {
			// Check for the case if control didn't receive welcome after sending hello for 100ms
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() && !clientState.receivedWelcome {
				log.Printf("Timeout while didn't receive welcome from the server: %v\n", netErr)
				done <- struct{}{}
				return
			} else if err == io.EOF {
				log.Println("Sever closed the connection")
				done <- struct{}{}
				return
			} else {
				log.Println("Failed to read from server")
				done <- struct{}{}
				return
			}
		} 

		log.Printf("Received reply type: %d\n", replyType[0]) // Debug log

		switch replyType[0] {
		case 2:
			// If receive welcome before sending hello
			if !clientState.sentHello {
				log.Println("Received Welcome before sending Hello. Disconnecting.")
				done <- struct{}{}
				return
			} else if clientState.receivedWelcome {
				// If we receive multiple welcome information
				log.Println("Received multiple Welcome messages. Disconnecting.")
				done <- struct{}{}
				return
			}


			// Hanlde the welcome message here
			welcomeMsg, err := handleWelcomeMessage(sock)
			if err != nil {
				log.Println("Failed to read welcome message: ", err)
				done <- struct{}{}
				return
			}
			
			// If succeed
			clientState.receivedWelcome = true
            welcomeCount++
			msgChan <- ServerMessage{Type: 2, Payload: welcomeMsg}
		case 3:
			// The case if client didn't send set station
			if !clientState.sentSetStation {
				log.Println("Received Announce before sending SetStation. Disconnecting.")
				done <- struct{}{}
				return
			}

			songName, err := handleAnnounceMessage(sock)
			if err != nil {
				log.Println("Failed to read announce message: ", err)
				done <- struct{}{}
				return
			}
			// If succeed
			msgChan <- ServerMessage{Type: 3, Payload: songName}
		case 4:
			invalidMsg, err := handleInvalidMessage(sock)
			if err != nil {
				log.Println("Failed to read the invalid message: ", err)
				done <- struct{}{}
				return
			}
			sock.Close()
			os.Exit(1)
			// If succeed
			msgChan <- ServerMessage{Type: 4, Payload: invalidMsg}
		default:
			log.Println("Received unknown message from server, closing connection")
			done <- struct{}{}
			return
		}
	}
}

func handleWelcomeMessage(sock net.Conn) (*protocol.Welcome, error){
		// Declare a buffer to receive welcome message
		welcomeBytes := make([]byte, 2)

		// Set read deadline
		sock.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		// Read the welcome message from server
		_, err := io.ReadFull(sock, welcomeBytes)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return nil, fmt.Errorf("timeout when reeding welcome message: %w", err)
			} else {
				return nil, fmt.Errorf("\nfailed to read welcome message from server: %w", err)
			}	
		}
		
		// Reset deadline
		sock.SetReadDeadline(time.Time{})

		welcome := &protocol.Welcome{
			ReplyType: 2,  // The replyType is already known (it was 2), so we set it here
		}
		buf := bytes.NewReader(welcomeBytes)
		err = binary.Read(buf, binary.BigEndian, &welcome.NumStations)
		if err != nil {
			return nil, fmt.Errorf("\nFailed to unmarshal numstations from server: %w", err)
		}

		return welcome, nil
}

func handleAnnounceMessage(sock net.Conn) (string, error){
	songNameSizeByte := make([]byte, 1)

	// Set read deadline
	sock.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

	_, err := sock.Read(songNameSizeByte)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return "", fmt.Errorf("\ntimeout when reeding announce message: %w", err)
		} else {
			return "", fmt.Errorf("\nfailed to read song name size: %w", err)
		}
	}

	// Get the size of the song, and assign the memory
	songNameSize := int(songNameSizeByte[0])
	songNameByte := make([]byte, songNameSize)
	// Read the name of the song
	_, err = io.ReadFull(sock, songNameByte)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return "", fmt.Errorf("\ntimeout when reeding announce message: %w", err)
		} else {
			return "", fmt.Errorf("\nfailed to read song name: %w", err)
		}
	}

	// Reset deadline
	sock.SetReadDeadline(time.Time{})

	songName := string(songNameByte)
	log.Printf("New song announced: %s\n", songName)
	return songName, nil
}

func handleInvalidMessage(sock net.Conn) (string, error) {
	// Trying to get the size of reply string
	invalidStringBytes := make([]byte, 1)

	// Set read deadline
	sock.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

	_, err := sock.Read(invalidStringBytes)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return "", fmt.Errorf("\ntimeout when reeding invalid message: %w", err)
		} else {
			return "", fmt.Errorf("\nfailed to read the invalid bytes: %w", err)
		}
	}

	// Get the size of invalid string, and read them from the server
	InvalidStringSize := int(invalidStringBytes[0])
	InvalidStringBytes := make([]byte, InvalidStringSize)

	_, err = io.ReadFull(sock, InvalidStringBytes)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return "", fmt.Errorf("\ntimeout when reeding invalid message: %w", err)
		} else {
			return "", fmt.Errorf("\nfailed to read the name of invalid song: %w", err)
		}
	}

	// Reset deadline
	sock.SetReadDeadline(time.Time{})
	InvalidString := string(InvalidStringBytes)
	log.Printf("Invalid command: %s\n", InvalidString)
	return InvalidString, nil
}
