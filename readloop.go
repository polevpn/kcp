package kcp

import (
	"github.com/pkg/errors"
)

func (s *UDPSession) readLoop() {
	buf := make([]byte, mtuLimit)
	for {
		if n, err := s.conn.Read(buf); err == nil {
			s.packetInput(buf[:n])
		} else {
			s.notifyReadError(errors.WithStack(err))
			return
		}
	}
}
