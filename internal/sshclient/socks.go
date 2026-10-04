package sshclient

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
)

var errSOCKS = errors.New("invalid SOCKS request")

// Exact reads leave pipelined application bytes on the socket. Domains are
// deliberately not resolved here: direct-tcpip performs destination-side DNS.
func socksRequest(r io.Reader, w io.Writer) (string, byte, error) {
	var header [2]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return "", 1, err
	}
	if header[0] != 5 || header[1] == 0 {
		return "", 1, errSOCKS
	}
	var methods [255]byte
	if _, err := io.ReadFull(r, methods[:int(header[1])]); err != nil {
		return "", 1, err
	}
	offered := false
	for _, method := range methods[:int(header[1])] {
		if method == 0 {
			offered = true
		}
	}
	method := byte(255)
	if offered {
		method = 0
	}
	if _, err := w.Write([]byte{5, method}); err != nil {
		return "", 1, err
	}
	if !offered {
		return "", 0, errSOCKS
	}
	var request [4]byte
	if _, err := io.ReadFull(r, request[:]); err != nil {
		return "", 1, err
	}
	if request[0] != 5 || request[2] != 0 {
		return "", 1, errSOCKS
	}
	if request[1] != 1 {
		return "", 7, errSOCKS
	}
	var host string
	switch request[3] {
	case 1, 4:
		size := 4
		if request[3] == 4 {
			size = 16
		}
		var address [16]byte
		if _, err := io.ReadFull(r, address[:size]); err != nil {
			return "", 1, err
		}
		host = net.IP(address[:size]).String()
	case 3:
		var length [1]byte
		if _, err := io.ReadFull(r, length[:]); err != nil {
			return "", 1, err
		}
		if length[0] == 0 {
			return "", 1, errSOCKS
		}
		var domain [255]byte
		if _, err := io.ReadFull(r, domain[:int(length[0])]); err != nil {
			return "", 1, err
		}
		host = string(domain[:int(length[0])])
		if strings.ContainsAny(host, ":[]") {
			return "", 1, errSOCKS
		}
		for _, b := range []byte(host) {
			if b <= 32 || b >= 127 {
				return "", 1, errSOCKS
			}
		}
	default:
		return "", 8, errSOCKS
	}
	var port [2]byte
	if _, err := io.ReadFull(r, port[:]); err != nil {
		return "", 1, err
	}
	value := binary.BigEndian.Uint16(port[:])
	if value == 0 {
		return "", 1, errSOCKS
	}
	return net.JoinHostPort(host, strconv.Itoa(int(value))), 0, nil
}

func socksReply(w io.Writer, code byte) error {
	// SSH does not expose the bound destination socket; use a neutral address.
	_, err := w.Write([]byte{5, code, 0, 1, 0, 0, 0, 0, 0, 0})
	return err
}
