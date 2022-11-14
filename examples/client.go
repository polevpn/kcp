package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/pion/dtls/v2"
	"github.com/polevpn/kcp"
)

func handleConn(sess *kcp.UDPSession) {

	for {

		sess.Write([]byte("hello"))

		buf := make([]byte, 4096)
		n, err := sess.Read(buf)

		if err != nil {
			fmt.Println("read fail,err=", err)
		}

		fmt.Println(string(buf[:n]))

		time.Sleep(time.Second * 5)

	}

}

func main() {
	// Prepare the IP to connect to
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 4444}

	// Prepare the configuration of the DTLS connection
	config := &dtls.Config{
		InsecureSkipVerify: false,
	}
	// Connect to a DTLS server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	defer cancel()

	dtlsConn, err := dtls.DialWithContext(ctx, "udp", addr, config)

	if err != nil {
		fmt.Println("dail fail,", err)
		return
	}
	defer dtlsConn.Close()

	sess, err := kcp.NewConn(0, dtlsConn.RemoteAddr(), 0, 0, dtlsConn)

	if err != nil {
		fmt.Println("create kcp session fail,", err)
		return
	}

	// Simulate a chat session
	handleConn(sess)
}
