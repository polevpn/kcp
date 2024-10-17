package main

import (
	"crypto/tls"
	"fmt"
	"net"

	"github.com/pion/dtls/v3"
	"github.com/polevpn/kcp"
)

func handleConn(sess *kcp.UDPSession) {

	for {

		buf := make([]byte, 4096)
		_, err := sess.Read(buf)

		if err != nil {
			fmt.Println("read fail,err=", err)
			break
		}

		sess.Write([]byte("2"))
	}

}

func main() {
	// Prepare the IP to connect to
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 4444}

	certificate, err := tls.LoadX509KeyPair("./keys/server.crt", "./keys/server.key")

	if err != nil {
		return
	}

	// Prepare the configuration of the DTLS connection
	config := &dtls.Config{
		Certificates: []tls.Certificate{certificate},
		MTU:          1400,
	}

	listener, err := kcp.Listen(addr, config)

	if err != nil {
		fmt.Println("kcp listen fail,", err)
		return
	}

	defer listener.Close()

	fmt.Println("Listening")

	for {
		// Wait for a connection.
		conn, err := listener.Accept()

		if err != nil {
			fmt.Println("kcp accept fail,", err)
			continue
		}

		conn.SetNoDelay(1, 10, 2, 0)
		conn.SetWindowSize(4096, 4096)

		go handleConn(conn)
	}
}
