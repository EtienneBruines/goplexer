package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v2"
)

var activeConnections int64

func main() {
	// Phase 1, read the config file
	f, err := ioutil.ReadFile("settings.yaml")
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal(f, &CurrentSettings)
	if err != nil {
		panic(err)
	}

	// Phase 2, start the server
	fmt.Println("Listening on", CurrentSettings.Server.Listen)

	addr, err := net.ResolveTCPAddr("tcp", CurrentSettings.Server.Listen)
	if err != nil {
		panic(err)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}

	// Phase 3, start handling connections
	for {
		// To prevent overloading
		for activeConnections >= int64(CurrentSettings.Server.MaxConnections) {
			time.Sleep(time.Microsecond)
		}

		// Handle it
		tcp, err := listener.AcceptTCP()
		if err != nil {
			continue
		}

		atomic.AddInt64(&activeConnections, 1)
		go handleConn(tcp)
	}
}

func lookup(buffer []byte) (string, error) {
	buffer = bytes.ToLower(buffer)

	for _, s := range CurrentSettings.Services {
		if bytes.HasPrefix(buffer, []byte(s.Keyword)) {
			return s.Location, nil
		}

		if len(s.KeywordBytes) > 0 && bytes.HasPrefix(buffer, s.KeywordBytes) {
			return s.Location, nil
		}
	}

	return "", errors.New("unable to figure out protocol")
}

func debug(i ...interface{}) {
	if !CurrentSettings.Server.Debug {
		return
	}

	log.Println(i...)
}

const bufferSize = 512 * 1024

func handleConn(tcp *net.TCPConn) {
	defer func() {
		atomic.AddInt64(&activeConnections, -1)
	}()

	debug("Accepted:", tcp.RemoteAddr().String())

	var (
		inward_incoming  = make([]byte, bufferSize)
		outward_incoming = make([]byte, bufferSize)

		inward_incomingN  int
		inward_outgoingN  int
		outward_incomingN int
		outward_outgoingN int

		err error

		external *net.Conn

		wait = make(chan struct{})
		stop = func() {
			if external != nil {
				(*external).Close()
			}

			tcp.Close()

			defer func() {
				recover()
			}()
			close(wait)
		}
	)

	go func() {
		for {
			inward_incomingN, err = tcp.Read(inward_incoming)
			// Phase 1, read the stuff we get from client
			if err != nil {
				if err == io.EOF {
					debug("Closed connection to client")
					stop()
					return
				} else {
					debug("Error reading inward_incoming:", err)
					stop()
					return
				}
			}

			// Phase 1.1 -> create connection to backend provider
			if external == nil {
				backendAddr, err := lookup(inward_incoming[:inward_incomingN])
				if err != nil {
					debug("Lookup error:", err)
					debug("(we might not know what to do with", inward_incoming[:inward_incomingN], ")")
					stop()
					return // maybe we need to read additional stuff? todo not failsafe, we might discard some important stuff
				}

				ext, err := net.Dial("tcp", backendAddr)
				if err != nil {
					debug("Error:", err)
					stop()
					return
				}

				external = &ext
			}

			if external == nil {
				continue // never continue without an outgoing connection
			}

			// Phase 2, write everything we receive to the backend provider
			if inward_incomingN > 0 {
				// Phase 2a, output to fmt
				debug(">", string(inward_incoming[:inward_incomingN]))

				// Phase 2b, proxy everything to that external server
				inward_outgoingN, err = (*external).Write(inward_incoming[:inward_incomingN])
				if err != nil {
					debug("Error writing inward_outgoing:", err)
					stop()
					return
				}

				inward_incomingN -= inward_outgoingN
			}
		}
	}()

	go func() {
		for {
			if external == nil {
				time.Sleep(time.Microsecond)
				continue
			}

			outward_incomingN, err = (*external).Read(outward_incoming)
			// Phase 3, read things from the backend provider
			if err != nil {
				if err == io.EOF {
					debug("Closed connection to backend")
					stop()
					return
				} else {
					debug("Error reading outward_incoming:", err)
					stop()
					return
				}
			}

			// Phase 4, write everything we just read to the client
			if outward_incomingN > 0 {
				// Phase 4a, output to fmt
				debug("<", string(outward_incoming[:outward_incomingN]))

				// Phase 4b, proxy everything to the client
				outward_outgoingN, err = tcp.Write(outward_incoming[:outward_incomingN])
				if err != nil {
					debug("Error writing outward_outgoing:", err)
					stop()
					return
				}

				outward_incomingN -= outward_outgoingN
			}
		}
	}()

	// wait before returning
	<-wait
}

type Settings struct {
	Server struct {
		Listen         string
		Debug          bool
		MaxConnections int
	}
	Services []struct {
		Type         string
		Keyword      string
		KeywordBytes []byte
		Location     string
	}
}

var CurrentSettings Settings
