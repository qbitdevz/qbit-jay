//go:build !unix

package server

import (
	"fmt"
	"net"
	"os"
)

func listenSecureUnixSocket(string) (net.Listener, os.FileInfo, error) {
	return nil, nil, fmt.Errorf("Unix sockets are not supported on this platform")
}

func secureUnixSocketPath(socketPath string) (string, error) {
	return "", fmt.Errorf("qbit-jay server: Unix sockets are not supported on this platform")
}
