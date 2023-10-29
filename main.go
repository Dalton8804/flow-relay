package main

import (
	"fmt"
	"log"
	"net"
)

type SyncRoom struct {
	Sender   net.Conn
	Receiver net.Conn
	Code     string
}

var clients = make(map[net.Conn]string)
var SyncRooms = make(map[string]SyncRoom)

func main() {
	listen, err := net.Listen("tcp", ":12345")
	if err != nil {
		fmt.Println("Error listening:", err)
		return
	}
	defer listen.Close()
	fmt.Println("Server listening on :12345")

	for {
		conn, err := listen.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		go handleRequest(conn)
	}
}

func handleRequest(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 1024)

	n, err := conn.Read(buf)
	if err != nil {
		log.Fatal("Error reading from connection:", err)
	}
	log.Printf("Received %d bytes: %s", n, string(buf[:n]))
	if string(buf[:n]) == "NEW" {
		newCode := createNewSyncRoom(conn)
		conn.Write([]byte(fmt.Sprintf("Have your receiver run\n`flow %s <folder-path>`", newCode)))
	} else {
		joinSyncRoom(conn, string(buf[:n]))
	}

	messageChannel := make(chan []byte)

	go watchForMessage(conn, messageChannel)

	for {
		select {
		case message := <-messageChannel:
			fmt.Println(string(message))
			if room, ok := SyncRooms[clients[conn]]; ok {
				if conn == room.Sender {
					fmt.Println("Sending message to receiver")
					room.Receiver.Write(message)
				} else {
					fmt.Println(string(message))
				}
			}
		}
	}
}

func watchForMessage(conn net.Conn, messageChannel chan []byte) {
	for {
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			log.Println("Client Connection closed.")
			if room, ok := SyncRooms[clients[conn]]; ok {
				if conn == room.Receiver {
					room.Sender.Write([]byte("Receiver disconnected"))
					SyncRooms[clients[conn]] = SyncRoom{Sender: room.Sender, Code: room.Code}
					delete(clients, conn)
				} else {
					if room.Receiver != nil {
						room.Receiver.Write([]byte("Sender disconnected"))
						room.Receiver.Close()
					}
					delete(SyncRooms, clients[conn])
					delete(clients, conn)
				}
			}
			break
		}
		messageChannel <- buf[:n]
	}
}

func getUniqueRoomCode() string {
	// return fmt.Sprintf("%d-%d-%d", rand.Intn(8999)+1000, rand.Intn(8999)+1000, rand.Intn(8999)+1000)
	return "1234"
}

func createNewSyncRoom(conn net.Conn) string {
	newCode := getUniqueRoomCode()
	_, ok := SyncRooms[newCode]
	log.Printf("Creating room code `%s`", newCode)
	for ok {
		newCode = getUniqueRoomCode()
		_, ok = SyncRooms[newCode]
		log.Println("Regenerating room code")
	}
	SyncRooms[newCode] = SyncRoom{Sender: conn, Code: newCode}
	clients[conn] = newCode
	log.Println("Sender connected")
	return newCode
}

func joinSyncRoom(conn net.Conn, code string) error {
	log.Printf("Attempting to join room `%s`", code)
	if room, ok := SyncRooms[code]; ok {
		if room.Receiver != nil {
			log.Printf("Room `%s` already has a receiver", code)
			conn.Write([]byte("There is already a receiver for this room"))
			conn.Close()
			return nil
		}
		room.Receiver = conn
		SyncRooms[code] = room
		clients[conn] = code
		room.Receiver.Write([]byte(fmt.Sprintf("Connected to room %s", code)))
		room.Sender.Write([]byte("Receiver connected"))
		log.Printf("Room `%s` exists", code)
	} else {
		log.Printf("Room `%s` does not exist", code)
		conn.Write([]byte(fmt.Sprintf("Room `%s` does not exist", code)))
		conn.Close()
	}
	return nil
}
