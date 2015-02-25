package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	goserial "github.com/ungerik/goserial"
)

var (
	running bool

	aliveTimer    time.Time
	aliveTimerMtx sync.Mutex
)

func ShowHelp() {
	fmt.Println("exit                          Terminates test application")
}

func BytePassThru(run *bool, source io.Reader, address *net.UDPAddr, id string) {

	target, err := net.DialUDP("udp", nil, address)
	if err != nil {
		panic(err)
	}

	buffer := []byte{0}
	target.SetWriteBuffer(1)

	for *run {
		_, err := source.Read(buffer)
		if err == nil {
			count, err := target.Write(buffer)
			if err != nil {
				fmt.Println("Write error: ", err)
			}
			fmt.Println(id, ": ", buffer[0], " (wrote ", count, " bytes)")
		}
	}

	target.Close()
}

func Passthru(address *net.UDPAddr, isLandingEngaged *bool) {

	config := &goserial.Config{Name: "/dev/ttyUSB0", Baud: 57600}
	source, err := goserial.OpenPort(config)
	if err != nil {
		panic(err)
	}
	fmt.Println("Routing to " + address.String())

	passthru := true
	go BytePassThru(&passthru, source, address, "GPS->UDP")

	for passthru {

		aliveTimerMtx.Lock()
		if time.Now().Sub(aliveTimer).Seconds() > 30 {
			fmt.Println("Stopping, no alives for 30secs")
			passthru = false
		}
		aliveTimerMtx.Unlock()
	}

	source.Close()
	*isLandingEngaged = false
}

func ConnectionServer() {
	addr := net.UDPAddr{
		Port: 4242,
		//IP:   net.ParseIP("192.168.42.1"),
		IP: net.ParseIP("10.0.0.68"),
	}

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
					aliveTimerMtx.Lock()
					aliveTimer = time.Now()
					aliveTimerMtx.Unlock()

					// ------ Command: interrupt -------
				} else if command == "interrupt" {

				}
			} else {

				// ------ Command: landing -------
				if command == "landing" {
					fmt.Println("Starting landing sequence")
					// Start GPS passthru
					isLandingEngaged = true
					aliveTimerMtx.Lock()
					aliveTimer = time.Now()
					aliveTimerMtx.Unlock()
					go Passthru(remoteAddress, &isLandingEngaged)
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
