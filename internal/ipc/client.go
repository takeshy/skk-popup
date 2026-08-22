package ipc

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// IsDaemonRunning reports whether another wl-skk daemon is accepting
// commands on the socket.
func IsDaemonRunning(socketPath string) bool {
	conn, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// RemoveStaleSocket deletes a leftover socket file that no daemon is
// serving. It returns the path even if removal was unnecessary.
func RemoveStaleSocket(socketPath string) string {
	if IsDaemonRunning(socketPath) {
		return socketPath
	}
	_ = os.Remove(socketPath)
	return socketPath
}

// RunClientCommand sends a single command to the daemon and prints the
// response. It returns an error both for transport failures and for
// "error:" responses.
func RunClientCommand(command string) error {
	if !IsValidCommand(command) {
		return fmt.Errorf("unknown command %q (expected toggle|show|hide|quit)", command)
	}
	socketPath := SocketPath()
	conn, err := net.DialTimeout("unix", socketPath, time.Second)
	if err != nil {
		return fmt.Errorf("wl-skk daemon is not running (%v)", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	if _, err := conn.Write([]byte(command + "\n")); err != nil {
		return fmt.Errorf("failed to send command: %w", err)
	}
	response, err := readLine(conn)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	if strings.HasPrefix(response, "error:") {
		return fmt.Errorf("%s", strings.TrimSpace(strings.TrimPrefix(response, "error:")))
	}
	fmt.Println(response)
	return nil
}
