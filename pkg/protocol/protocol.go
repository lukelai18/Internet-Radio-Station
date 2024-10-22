package protocol

import (
	"bytes"
	"encoding/binary"
)

// Struct for Hello
type Hello struct {
	CommandType uint8
	UdpPort     uint16
}

// Struct for SetStation
type SetStation struct {
	CommandType uint8
	StationNumber  uint16
}

// Struct for Welcome
type Welcome struct {
	ReplyType 	  uint8
	NumStations   uint16
}

// Struct for Announce
type Announce struct {
	ReplyType 	  uint8
	SongnameSize  uint8
	Songname      string
}

// Struct for InvalidCommand
type InvalidCommand struct {
	ReplyType 	  uint8
	ReplyStringSize  uint8
	ReplyString    string
}

// Serialize the hello message
func(h *Hello) Marshal() ([]byte, error) {
	// Asign new space
	buf := new(bytes.Buffer)
	
	// Write the content of 'h' into buffer
	err := binary.Write(buf, binary.BigEndian, h.CommandType)
	if err != nil {
		return nil, err
	}
	
	err = binary.Write(buf, binary.BigEndian, h.UdpPort)
	if err != nil {
		return nil, err
	}

	// Return the byte array, and no error
	return buf.Bytes(), nil
}

// Deserialize the hello message
func UnMarshalHello(data []byte) (*Hello, error) {
	// Get the reader of data and initialize a 'h' to store the Hello struct
	buf := bytes.NewReader(data)
	h := &Hello{}

	// Put the data into h struct
	err := binary.Read(buf, binary.BigEndian, &h.CommandType)
	if err != nil {
		return nil, err
	}

	err = binary.Read(buf, binary.BigEndian, &h.UdpPort)
	if err != nil {
		return nil, err
	}

	// Return the deserialize hello struct
	return h, nil
}

// Serialize the setStation message
func(s *SetStation) Marshal() ([]byte, error) {
	// Asign new space
	buf := new(bytes.Buffer)
	
	// Write the content of 'h' into buffer
	err := binary.Write(buf, binary.BigEndian, s.CommandType)
	if err != nil {
		return nil, err
	}
	
	err = binary.Write(buf, binary.BigEndian, s.StationNumber)
	if err != nil {
		return nil, err
	}

	// Return the byte array, and no error
	return buf.Bytes(), nil
}

// Deserialize the setStation message
func UnMarshalSetStation(data []byte) (*SetStation, error) {
	// Get the reader of data and initialize a 's' to store the SetStation struct
	buf := bytes.NewReader(data)
	s := &SetStation{}

	// Put the data into h struct
	err := binary.Read(buf, binary.BigEndian, &s.CommandType)
	if err != nil {
		return nil, err
	}

	err = binary.Read(buf, binary.BigEndian, &s.StationNumber)
	if err != nil {
		return nil, err
	}

	// Return the deserialize hello struct
	return s, nil
}

// Serialize the anounce message
func(a *Announce) Marshal() ([]byte, error) {
	// Asign new space
	buf := new(bytes.Buffer)
	
	// Write the content of 'a' into buffer
	err := binary.Write(buf, binary.BigEndian, a.ReplyType)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, a.SongnameSize)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, []byte(a.Songname))
	if err != nil {
		return nil, err
	}

	// Return the byte array, and no error
	return buf.Bytes(), nil
}

// Deserialize the announce message
func UnMarshalAnnounce(data []byte) (*Announce, error) {
	// Get the reader of data and initialize a 'h' to store the Hello struct
	buf := bytes.NewReader(data)
	a := &Announce{}

	// Put the data into a struct
	err := binary.Read(buf, binary.BigEndian, &a.ReplyType)
	if err != nil {
		return nil, err
	}

	err = binary.Read(buf, binary.BigEndian, &a.SongnameSize)
	if err != nil {
		return nil, err
	}

	// Read the songname based on songnameSize
	songnameBytes := make([]byte, a.SongnameSize)
	err = binary.Read(buf, binary.BigEndian, &songnameBytes)
	if err != nil {
		return nil, err
	}

	// Convert the Bytes to string
	a.Songname = string(songnameBytes)

	// Return the deserialize hello struct
	return a, nil
}

// Serialize the Welcome message
func(w *Welcome) Marshal() ([]byte, error) {
	// Asign new space
	buf := new(bytes.Buffer)
	
	// Write the content of 'w' into buffer
	err := binary.Write(buf, binary.BigEndian, w.ReplyType)
	if err != nil {
		return nil, err
	}
	
	err = binary.Write(buf, binary.BigEndian, w.NumStations)
	if err != nil {
		return nil, err
	}

	// Return the byte array, and no error
	return buf.Bytes(), nil
}

// Deserialize the welcome data
func UnMarshalWelcome(data []byte) (*Welcome, error) {
	// Initialize the struct
	buf := bytes.NewReader(data)
	w := &Welcome{}

	// Deserialize the data
	err := binary.Read(buf, binary.BigEndian, &w.ReplyType)
	if err != nil {
		return nil, err
	}

	err = binary.Read(buf, binary.BigEndian, &w.NumStations)
	if err != nil {
		return nil, err
	}

	// Return the deserialized data
	return w, nil
}

// Serialize the invalidCommand message
func(i *InvalidCommand) Marshal() ([]byte, error) {
	// Asign new space
	buf := new(bytes.Buffer)
	
	// Write the content of 'i' into buffer
	err := binary.Write(buf, binary.BigEndian, i.ReplyType)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, i.ReplyStringSize)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, []byte(i.ReplyString))
	if err != nil {
		return nil, err
	}

	// Return the byte array, and no error
	return buf.Bytes(), nil
}

// Deserialize the InvalidCommand message
func UnMarshalInvalidCommand(data []byte) (*InvalidCommand, error) {
	// Get the reader of data and initialize a 'h' to store the Hello struct
	buf := bytes.NewReader(data)
	i := &InvalidCommand{}

	// Put the data into a struct
	err := binary.Read(buf, binary.BigEndian, &i.ReplyType)
	if err != nil {
		return nil, err
	}

	err = binary.Read(buf, binary.BigEndian, &i.ReplyStringSize)
	if err != nil {
		return nil, err
	}

	// Read the songname based on reply string size
	replyStringBytes := make([]byte, i.ReplyStringSize)
	err = binary.Read(buf, binary.BigEndian, &replyStringBytes)
	if err != nil {
		return nil, err
	}

	// Convert the Bytes to string
	i.ReplyString = string(replyStringBytes)

	// Return the deserialize hello struct
	return i, nil
}