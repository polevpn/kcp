package main

import (
	"fmt"
	"net"

	"github.com/polevpn/kcp"
)

func handleConn(sess *kcp.UDPSession) {

	for {

		buf := make([]byte, 4096)
		n, err := sess.Read(buf)

		if err != nil {
			fmt.Println("read fail,err=", err)
			break
		}

		fmt.Println(string(buf[:n]))
		sess.Write(buf[:n])

	}

}

func main() {
	// Prepare the IP to connect to
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 4444}

	listener, err := kcp.Listen(addr, "./keys/server.crt", "./keys/server.key")

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

		conn.SetNoDelay(1, 10, 2, 1)

		go handleConn(conn)
	}

}
