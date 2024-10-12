package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/polevpn/kcp"
)

func handleConn(sess *kcp.UDPSession) {

	count := 0

	for {

		sess.Write([]byte("hello"))

		buf := make([]byte, 4096)
		n, err := sess.Read(buf)

		if err != nil {
			fmt.Println("read fail,err=", err)
		}

		fmt.Println(string(buf[:n]))

		count++

		if count > 5 {
			sess.Close()
			break
		}

		time.Sleep(time.Second * 1)

	}

}

func main() {
	// Prepare the IP to connect to
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 4444}

	// Connect to a DTLS server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	defer cancel()

	conn, err := kcp.DialWithContext(ctx, addr)

	if err != nil {
		fmt.Println("dail fail,", err)
		return
	}
	defer conn.Close()

	conn.SetNoDelay(1, 20, 2, 1)

	// Simulate a chat session
	handleConn(conn)
}
