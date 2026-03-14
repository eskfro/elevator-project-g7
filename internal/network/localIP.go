package network

import (
	"net"
	"strings"
)

var localIP_addr string

func localIP() (string, error) {
	if localIP_addr == "" {
		conn, err := net.DialTCP("tcp4", nil, &net.TCPAddr{IP: []byte{8, 8, 8, 8}, Port: 53})
		if err != nil {
			return "", err
		}
		defer conn.Close()
		localIP_addr = strings.Split(conn.LocalAddr().String(), ":")[0]
	}
	return localIP_addr, nil
}
