package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	bb "github.com/SpaceLeap/go-beaglebone"
)

var running bool

// to sync alive time stamps
var mtx sync.Mutex

func ShowHelp() {
	fmt.Println("exit                          Terminates test application")
}

func BytePassThru(run *bool, source *bb.UART, target *net.UDPConn, id string) {
	go func() {

		buffer := []byte{0}

		for *run {
			_, err := source.Read(buffer)
			if err != nil {
				fmt.Println("Read error: ", err)
			} else {
				fmt.Println(id, ": ", buffer[0])
				_, err = target.Write(buffer)
				if err != nil {
					fmt.Println("Write error: ", err)
				}
			}
		}
	}()
}

func Passthru(address *net.UDPAddr, aliveTimer *time.Time, isLandingEngaged *bool) {

	source, err := bb.NewUART(bb.UART2, bb.UART_BAUD_57600, bb.UART_BYTESIZE_8, bb.UART_PARITY_NONE, bb.UART_STOPBITS_1)
	if err != nil {
		panic(err)
	}
	client, err := net.DialUDP("udp", nil, address)
	if err != nil {
		panic(err)
	}

	passthru := true
	BytePassThru(&passthru, source, client, "GPS->UDP")

	for passthru {

		mtx.Lock()
		if time.Now().Sub(*aliveTimer).Seconds() > 30 {
			fmt.Println("Stopping, no alives for 30secs")
			passthru = false
		}
		mtx.Unlock()
	}

	client.Close()
	*isLandingEngaged = false
}

func ConnectionServer() {
	addr := net.UDPAddr{
		Port: 4242,
		//IP:   net.ParseIP("192.168.42.1"),
		IP: net.ParseIP("10.0.0.68"),
	}

	var aliveTimer time.Time

	isLandingEngaged := false

	for running {
		conn, err := net.ListenUDP("udp", &addr)
		if err != nil {
			panic(err)
		}

		var buf [1024]byte
		length, remoteAddress, err := conn.ReadFromUDP(buf[:])
		if err == nil {
			command := string(buf[:length])
			fmt.Println("Received command: " + command)

			// ------ Command: status -------
			if command == "status" {
				conn.WriteTo([]byte("landing: "+strconv.FormatBool(isLandingEngaged)), remoteAddress)
			} else if isLandingEngaged == true {

				// ------ Command: alive -------
				if command == "alive" {
					mtx.Lock()
					aliveTimer = time.Now()
					mtx.Unlock()

					// ------ Command: interrupt -------
				} else if command == "interrupt" {

				}
			} else {

				// ------ Command: landing -------
				if command == "landing" {
					fmt.Println("Starting landing sequence")
					// Start GPS passthru
					isLandingEngaged = true
					mtx.Lock()
					aliveTimer = time.Now()
					mtx.Unlock()
					go Passthru(remoteAddress, &aliveTimer, &isLandingEngaged)
				}
			}
		}
		conn.Close()
	}
}

func main() {

	reader := bufio.NewReader(os.Stdin)

	running = true

	go ConnectionServer()

	fmt.Println("Enter help for a list of available commands")
	for running {
		fmt.Print("> ")
		command, _ := reader.ReadString('\n')
		command = strings.TrimSpace(command)

		if command == "help" {
			ShowHelp()
		} else if command == "exit" {
			running = false
		}
	}
}
