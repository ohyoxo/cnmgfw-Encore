package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

var (
	uuid = strings.ReplaceAll(os.Getenv("UUID"), "-", "")
	port = os.Getenv("PORT")
)

func main() {
	if uuid == "" {
		uuid = "b84a3458-e83a-4337-ada2-b303b6d2a841"
	}
	if port == "" {
		port = "3000"
	}

	log.Printf("listen: %s", port)
	server, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	upgrader := websocket.Upgrader{}

	for {
		conn, err := server.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		go handleConnection(conn, upgrader)
	}
}

func handleConnection(conn net.Conn, upgrader websocket.Upgrader) {
	defer conn.Close()

	ws, _, err := upgrader.Upgrade(conn)
	if err != nil {
		log.Printf("Upgrade error: %v", err)
		return
	}
	defer ws.Close()

	_, msg, err := ws.ReadMessage()
	if err != nil {
		log.Printf("Read message error: %v", err)
		return
	}

	// Validate UUID
	version := msg[0]
	id := msg[1:17]
	for i := 0; i < len(id); i++ {
		v, _ := parseHex(uuid[i*2 : i*2+2])
		if id[i] != byte(v) {
			return
		}
	}

	// Parse message
	i := int(msg[17]) + 19
	port := binary.BigEndian.Uint16(msg[i : i+2])
	i += 2
	atyp := msg[i]
	i++

	var host string
	switch atyp {
	case 1: // IPv4
		host = fmt.Sprintf("%d.%d.%d.%d", msg[i], msg[i+1], msg[i+2], msg[i+3])
		i += 4
	case 2: // Domain
		length := int(msg[i])
		i++
		host = string(msg[i : i+length])
		i += length
	case 3: // IPv6
		var parts []string
		for j := 0; j < 16; j += 2 {
			num := binary.BigEndian.Uint16(msg[i+j : i+j+2])
			parts = append(parts, fmt.Sprintf("%x", num))
		}
		host = strings.Join(parts, ":")
		i += 16
	default:
		return
	}

	log.Printf("conn: %s %d", host, port)

	// Send response
	err = ws.WriteMessage(websocket.BinaryMessage, []byte{version, 0})
	if err != nil {
		log.Printf("Write message error: %v", err)
		return
	}

	// Connect to target
	targetConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		log.Printf("Conn-Err: %v %s:%d", err, host, port)
		return
	}
	defer targetConn.Close()

	// Write remaining message
	remaining := msg[i:]
	if len(remaining) > 0 {
		_, err = targetConn.Write(remaining)
		if err != nil {
			log.Printf("Write to target error: %v", err)
			return
		}
	}

	// Pipe data between connections
	go func() {
		_, err := copyBuffer(targetConn, ws.UnderlyingConn())
		if err != nil {
			log.Printf("E1: %v", err)
		}
	}()
	_, err = copyBuffer(ws.UnderlyingConn(), targetConn)
	if err != nil {
		log.Printf("E2: %v", err)
	}
}

// Helper function to parse hex string to int
func parseHex(hex string) (int, error) {
	var result int
	_, err := fmt.Sscanf(hex, "%x", &result)
	return result, err
}

// Copy buffer between connections
func copyBuffer(dst net.Conn, src net.Conn) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				return written, ew
			}
			if nr != nw {
				return written, fmt.Errorf("short write")
			}
		}
		if er != nil {
			return written, er
		}
	}
}
