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
	"strings"
	"sync"
	"time"
)

type ClientMessage struct {
	Type    uint8
	Payload interface{}
}

type UserCommand struct {
    Command string
    Args    []string
}

// Struct for Client information
type ClientInfo struct {
	net.TCPConn
	conn 		net.Conn
	StationID  int
	UDPAddr  *net.UDPAddr
	UDPConn  *net.UDPConn
	Mutex    sync.Mutex
	TCPWriteChan    chan []byte	// Message over TCP
    UDPWriteChan    chan []byte // Message over UDP
    DisconnectChan  chan struct{} // Signal client disconnection
	HandshakeCompleted bool  // 
	HelloReceived bool
}

// Struct for Station
type Station struct {
	ID int
	FilePath string
	Clients  map[string]*ClientInfo
	CurrentOffset int64 	// Current position in the song
	Mutex	sync.Mutex		// Protect access to clients
}

// Hold all stations
var stations []*Station

func main(){
	if len(os.Args) < 3 {
		log.Fatalf("Usage: ./snowcast_server <listen port> <file0> [file1] [file2] not correct %s", os.Args[0])
	}

	// Get listen port
	listen_port := os.Args[1]
	// Get the number of stations
	file := os.Args[2:]
	numStation := len(file)

	log.Printf("The num of station is %d \n", numStation)

	startServer(listen_port, file, numStation)
}

// Initialize the server
func startServer(listen_port string, stationFiles []string, numStation int) {
	initializeStations(stationFiles)
	startStreaming()

	// Get TCP address
	TCPAddr, err := net.ResolveTCPAddr("tcp4", "127.0.0.1:"+listen_port)
	if err != nil {
		log.Fatal(err)
	}
		
	// Create listener
	listenSock, err := net.ListenTCP("tcp4", TCPAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer listenSock.Close()

	go waitForConnections(listenSock, numStation)

	go startServerCLI()
	
	// Block main goroutine
	select {}
}

func waitForConnections(listenSock *net.TCPListener, numStation int) {
	for {
		// Accpet the client connection
		clientSock, err := listenSock.AcceptTCP()
		if err != nil {
			log.Println("Error accepting connection:", err)
			continue
		}
		
		// If a client connect successfully, initialize a new client in the code below
		// Set up the UDP part in client
		clientUDPAddr := &net.UDPAddr{
			IP: clientSock.RemoteAddr().(*net.TCPAddr).IP,
			Port: 0, // Will be set after receiving Hello
		}
		// Create a new client
		client := &ClientInfo{
			conn: clientSock,
			StationID: -1,	// Set a default value for 0
			UDPAddr: clientUDPAddr,
			TCPWriteChan:   make(chan []byte, 100),
        	UDPWriteChan:   make(chan []byte, 1600),
        	DisconnectChan: make(chan struct{}),
		}

		go tcpWriter(client)
		go udpWriter(client)

		go handleClient(client, numStation)
	}
}

// Initialize the stations list
func initializeStations(stationFiles []string) {
	for i, file := range stationFiles {
		curStation := &Station{
			ID : i,
			FilePath: file,
			Clients : make(map[string]*ClientInfo),
		}

		// Log when each station is ready
		log.Printf("The song %s is ready at station %d\n", curStation.FilePath, curStation.ID)

		stations = append(stations, curStation)
	}
}

func handleClient(client *ClientInfo, numStation int) {
	// defer client.conn.Close()
	defer func() {
		removeClientFromStation(client, client.StationID)
		client.conn.Close()
		if client.UDPConn != nil {
            client.UDPConn.Close()
        }
        close(client.DisconnectChan)
    }()

	// Set a 100ms deadline
	client.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

	for {	
		// Try to get the command type fisrtly
		commandType := make([]byte, 1)
		_, err := client.conn.Read(commandType)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() && !client.HelloReceived {
				log.Printf("Timeout while didn't receive hello message from the client after connection %s: %v\n", 
					client.conn.RemoteAddr(), netErr)
					client.DisconnectChan <- struct{}{}
					return
			} else {
				log.Printf("Client %s disconnected: %v\n", client.conn.RemoteAddr(), err)
				client.DisconnectChan <- struct{}{}
				return
			}
		}

		client.conn.SetReadDeadline(time.Time{})
		log.Printf("Received client message type: %d\n", commandType[0]) // Debug log

		switch commandType[0] {
		// Hello message
		case 0:
			// Handling the mutiple hello
			if client.HandshakeCompleted {
				sendInvalidCommand(client, "Send hello command more than once")
				// client.DisconnectChan <- struct{}{}
				return
			}

			_, err := handleHelloMessage(client)
			if err != nil {
				log.Printf("Failed to receive hello message from client: %v\n", err)
				// client.DisconnectChan <- struct{}{}
				return
			}

			welcome := &protocol.Welcome{
				ReplyType: 2,
				NumStations: uint16(numStation),
			}	
				
			welcomeBytes, err := welcome.Marshal()
			if err != nil {
				log.Printf("Failed to marshal welcome message: %v\n", err)
				// client.DisconnectChan <- struct{}{}
				return
			}
			
            client.TCPWriteChan <- welcomeBytes
            log.Println("Welcome message sent to client")

            // Mark handshake as completed
            client.HandshakeCompleted = true
			// client.TCPWriteChan <- welcomeBytes
		// Set station
		case 1:
			// Handle send the before the hello
			if !client.HandshakeCompleted {
				sendInvalidCommand(client, "Send set staion command before hello")
				// client.DisconnectChan <- struct{}{}
				return
			}
			_, err := handleSetStation(client)
			if err != nil {
				log.Printf("Failed to process set station message: %v\n", err)
				// client.DisconnectChan <- struct{}{}
				return
			}
		default:
			sendInvalidCommand(client, "Unknown command")
			// client.DisconnectChan <- struct{}{}
			return
		}
	}
}

// Handling the case to write data using TCP protocol
func tcpWriter(client *ClientInfo) {
	// defer client.conn.Close()
	for {
		select {
		case data, ok := <-client.TCPWriteChan:
			if !ok {
                return
            }
			_, err := client.conn.Write(data)
			if err != nil {
				log.Printf("Failed to write data to client through TCP %v\n", err)
				// client.DisconnectChan <- struct{}{}
				return
			}
		case <-client.DisconnectChan:
			return
		}
	}
}

// Sending data through UDP protocol
func udpWriter(client *ClientInfo) {
	for {
		select {
		case data, ok := <-client.UDPWriteChan:
			if !ok {
				return
			}

			if client.UDPConn == nil {
				log.Printf("UDP didn't set up for client %s\n", client.conn.RemoteAddr().String())
			}
			_, err := client.UDPConn.Write(data)
			if err != nil {
				log.Printf("Failed to write data to client.%s %v\n", client.conn.RemoteAddr().String(), err)
				// client.DisconnectChan <- struct{}{}
				return
			}
		case <-client.DisconnectChan:
			return
		}
	}
}


// Handle the invalid commands from client
func sendInvalidCommand(client *ClientInfo, message string) {
	invalid := &protocol.InvalidCommand{
		ReplyType : 4,
		ReplyStringSize: uint8(len(message)),
		ReplyString: message,
	}

	invalidBytes, err := invalid.Marshal()
	if err != nil {
        log.Printf("Failed to marshal InvalidCommand: %v\n", err)
        return
    }
	client.TCPWriteChan <- invalidBytes
	// client.TCPWriteChan <- ClientMessage{Type: 4, Payload: invalidBytes}
}

// Handle the hello message from client
func handleHelloMessage(client *ClientInfo) (*protocol.Hello, error) {
	helloPortBytes := make([]byte, 2)

	// Set read deadline
	client.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

	_, err := io.ReadFull(client.conn, helloPortBytes)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			log.Printf("Timeout while didn't receive message back from clieny: %v\n", netErr)
			sendInvalidCommand(client, "Timeout when reading hello message")
		} else {
			log.Printf("Failed to read first hello message from client: %v\n", err)
			sendInvalidCommand(client, "Failed to read Hello UDP port")
		}
		return nil, err
	}

	// Reset deadline
	client.conn.SetReadDeadline(time.Time{})

	hello := &protocol.Hello{
		CommandType: 0,
	}
	buf := bytes.NewReader(helloPortBytes)
	err = binary.Read(buf, binary.BigEndian, &hello.UdpPort)
	if err != nil {
		log.Printf("Failed to unmarshal Hello message: %v\n", err)
        sendInvalidCommand(client, "Failed to parse Hello message")
		return nil, err
	}

	// Update client's UDP port.
	client.UDPAddr.Port = int(hello.UdpPort)
	clientUDPConn, err := net.DialUDP("udp4", nil, client.UDPAddr)
	if err != nil {
        log.Printf("Failed to dial UDP address %v: %v\n", client.UDPAddr, err)
        sendInvalidCommand(client, "Failed to set up UDP connection")
        return nil, err
    }
	client.UDPConn = clientUDPConn

	// Mark it finishes handshake process
	client.HelloReceived = true
	log.Printf("the UDP port from hello is %d, sent to station: %d\n", hello.UdpPort, client.StationID)
	return hello, nil
}

// Handle the set station command from client
func handleSetStation(client *ClientInfo) (int, error) {
	stationNumBytes := make([]byte, 2)

	// Set read deadline
	client.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

	_, err := io.ReadFull(client.conn, stationNumBytes)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			log.Printf("Timeout while handling set station message: %v\n", err)
			sendInvalidCommand(client, "Timeout while handling set station message")
		} else {
			log.Printf("Failed to read the station number: %v\n", err)
			sendInvalidCommand(client, "Failed to read the station number")
		}	
		return -1, err
	}

	// Reset deadline
	client.conn.SetReadDeadline(time.Time{})

	stationNum := binary.BigEndian.Uint16(stationNumBytes)
	if int(stationNum) >= len(stations) || int(stationNum) < 0 {
		// TODO: finish the sendInvalidCommand function
		sendInvalidCommand(client, "The station number is invalid")

		return -1, fmt.Errorf("invalid station number: %d", stationNum)
	}

	// Update client's station assignment
	client.Mutex.Lock()
	oldStationNum := client.StationID
	client.StationID = int(stationNum)
	client.Mutex.Unlock()
	
	if oldStationNum != -1 {
        removeClientFromStation(client, oldStationNum)
    }
    addClientToStation(client, client.StationID)
	
	log.Printf("Succeed in update the station number from %d to %d, the current address is %s\n", 
		oldStationNum, stationNum, client.conn.RemoteAddr())
	
	announceNewSong(stations[client.StationID])
	
	return int(stationNum), nil
}

// Remove the client from the map of the Station
func removeClientFromStation(client *ClientInfo, stationID int) {
	log.Printf("The station ID we need to remove is %d\n", stationID)
	if stationID == -1 {
		return
	}

	// Get the station we need
	station := stations[stationID]

	// Lock the station's mutex
	station.Mutex.Lock()
	defer station.Mutex.Unlock()

	// Remove the client from the map
	clientAddress := client.conn.RemoteAddr().String()
	delete(station.Clients, clientAddress)

	// Log the removal
	// log.Printf("Client %s removed from station %d ", clientAddress, stationID)
}

// Add the client into the map of the station
func addClientToStation(client *ClientInfo, stationID int) {
	// Get the station we need
	station := stations[stationID]

	// Lock the station's mutex to protect data
	station.Mutex.Lock()
	defer station.Mutex.Unlock()

	// Add the client to the station's map
	clientAddress := client.conn.RemoteAddr().String()
	station.Clients[clientAddress] = client

	// Print the information
	// log.Printf("Client %s was added to station %d ", clientAddress, stationID)
}

// Start stream the song file and send them to Clients
func startStreaming() {
	for _,curStation := range stations {
		go streamStation(curStation)
	}
}

func streamStation(station *Station) {
	// Open the song file 
	file, err := os.Open(station.FilePath)
	if err != nil {
		log.Printf("Failed to open the file %s, with the error code: %v", station.FilePath, err)
		return
	}
	defer file.Close()

	// Log that the song is ready for streaming
	log.Printf("The song %s is now streaming at station %d\n", station.FilePath, station.ID)

	// Max packet size
	bufferSize := 1500
	buffer := make([]byte, bufferSize)

	// Calculate the sleeping time for single packet, make sure it transmission rate is 16kb/s
	sleepDuration := time.Duration(float64(bufferSize)/16384.0*float64(time.Second)) // ≈ 91.6ms

	for {
		// Read the data into the 16 kb buffer we declared before
		numOfBytes, err := file.Read(buffer)

		if err == io.EOF {
			// We've reached the end of the file, loop the song
			log.Printf("Reached EOF for %s, restarting the song", station.FilePath)
			// Reset the song
			// station.CurrentOffset = 0
			file.Seek(0, io.SeekStart)
			// Send new announce to clients
			announceNewSong(station)
			continue
		} else if err != nil {
			log.Printf("Error reading file %s: %v", station.FilePath, err)
			continue
		}

		// If there are no error, update the offset
		// station.CurrentOffset += int64(numOfBytes)

		// Send all the media data to clients
		station.Mutex.Lock()
		for _, curClient := range station.Clients {
			select {
			case curClient.UDPWriteChan <- buffer[:numOfBytes]:
			default:
				log.Printf("Cannot write data into udp write channel %s\n", 
					curClient.UDPConn.RemoteAddr().String())
				curClient.DisconnectChan <- struct{}{}
			}
		}
		station.Mutex.Unlock()

		time.Sleep(sleepDuration)
	}
}

// Announce a new song to client
func announceNewSong(station *Station) {
	// Initialize and marshal the data needed to be sent
	announceData := &protocol.Announce{
		ReplyType: 3,
		SongnameSize: uint8(len(station.FilePath)),
		Songname: station.FilePath,
	}
	announceDataBytes, err := announceData.Marshal()
	if err != nil {
		log.Printf("Failed to marshal the announce data: %v", err)
		return
	}

	// Lock the mutex of the station firstly
	station.Mutex.Lock()
	defer station.Mutex.Unlock()

	// Send the announce data to the clients in the station's list
	for _, curClient := range station.Clients {
		select {
		case curClient.TCPWriteChan <- announceDataBytes:
		default:
			log.Printf("Didn't write data to TCPWrite Channel %s\n", curClient.conn.RemoteAddr().String())
			curClient.DisconnectChan <- struct{}{}
		}
	}
}

// Read the user input information
func startServerCLI() {
	reader := bufio.NewReader(os.Stdin)
	for {
		// Try to get the input from current line, and remove the leading and trailing white space
		input, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Can not read the input from user: %v", err)
			continue
		}
		input = strings.TrimSpace(input)

		if input == "q" {
			// If input q, exit the system
			fmt.Println("Shut down the server")
			shutDownServer()
			os.Exit(0)
		} else if strings.HasPrefix(input, "p") {
			// If the input start from the p, separate them by the space between the characters
			args := strings.Fields(input)
			if len(args) == 1 {
				printStations()
			} else if len(args) == 2 {
				writeStationsToFile(args[1])
			} else {
				fmt.Println("Invalid usage for p command")
			}
		} else {
			fmt.Println("Invalid command")
		}
	}
}

// Print the information of stations, including the station ID, path of station
// and the IP address and port number
func printStations() {
	for _, curStation := range stations {
		curStation.Mutex.Lock()
		fmt.Printf("%d,%s", curStation.ID, curStation.FilePath)
		for _, curClient := range curStation.Clients {
			// Print the IP address and port number
			fmt.Printf(",%s:%d", curClient.UDPAddr.IP.String(), curClient.UDPAddr.Port)
		}
		fmt.Printf("\n")
		curStation.Mutex.Unlock()
	}
}

func writeStationsToFile(fileName string) {
	// Create or overwrite a new file
	file, err := os.Create(fileName)
	if err != nil {
		fmt.Println("Failed to create a file: ", err)
		return
	}

	defer file.Close()
	
	// Create a new writer
	writer := bufio.NewWriter(file)
	// Similar to the previous function, but write the content to the file
	for _, curStation := range stations {
		curStation.Mutex.Lock()
		fmt.Fprintf(writer, "%d,%s", curStation.ID, curStation.FilePath)
		for _, curClient := range curStation.Clients {
			fmt.Fprintf(writer, ",%s:%d", curClient.UDPAddr.IP.String(), curClient.UDPAddr.Port)	
		}
		fmt.Fprintf(writer,"\n")
		curStation.Mutex.Unlock()
	}
	// Ensure that data is written to file
	err = writer.Flush()
	if err != nil {
		fmt.Println("Failed to write data into file: ", err)
	}
}

// Close all the connected clients
func shutDownServer() {
	for _, curStation := range stations {
		curStation.Mutex.Lock()
		for _, curCLient := range curStation.Clients {
			curCLient.conn.Close()
			if curCLient.UDPConn != nil {
                curCLient.UDPConn.Close()
            }
		}
		curStation.Mutex.Unlock()
	}
}