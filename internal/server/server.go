package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/AvengeMedia/danksearch/internal/log"
	"github.com/AvengeMedia/danksearch/internal/server/models"
)

const APIVersion = 1

type ServerInfo struct {
	APIVersion int `json:"apiVersion"`
}

func getSocketDir() string {
	if runtime := os.Getenv("XDG_RUNTIME_DIR"); runtime != "" {
		return runtime
	}

	if os.Getuid() == 0 {
		if _, err := os.Stat("/run"); err == nil {
			return "/run/danksearch"
		}
		return "/var/run/danksearch"
	}

	return os.TempDir()
}

func GetSocketPath() string {
	return filepath.Join(getSocketDir(), fmt.Sprintf("danksearch-%d.sock", os.Getpid()))
}

func cleanupStaleSockets() {
	dir := getSocketDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "danksearch-") || !strings.HasSuffix(entry.Name(), ".sock") {
			continue
		}

		pidStr := strings.TrimPrefix(entry.Name(), "danksearch-")
		pidStr = strings.TrimSuffix(pidStr, ".sock")
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			socketPath := filepath.Join(dir, entry.Name())
			os.Remove(socketPath)
			log.Debugf("Removed stale socket: %s", socketPath)
			continue
		}

		err = process.Signal(syscall.Signal(0))
		if err != nil {
			socketPath := filepath.Join(dir, entry.Name())
			os.Remove(socketPath)
			log.Debugf("Removed stale socket: %s", socketPath)
		}
	}
}

type UnixServer struct {
	router   *Router
	listener net.Listener
}

func NewUnix(router *Router) *UnixServer {
	return &UnixServer{
		router: router,
	}
}

func (s *UnixServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	srvInfoData, _ := json.Marshal(ServerInfo{
		APIVersion: APIVersion,
	})
	conn.Write(srvInfoData)
	conn.Write([]byte("\n"))

	scanner := bufio.NewScanner(conn)
	// Increase buffer size to handle large requests (default is 64KB max)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // 10MB max
	if scanner.Scan() {
		line := scanner.Bytes()

		var req models.Request
		if err := json.Unmarshal(line, &req); err != nil {
			models.RespondError(conn, 0, "invalid json")
			return
		}

		s.router.RouteRequest(conn, req)
	}
}

func (s *UnixServer) Start() error {
	cleanupStaleSockets()

	socketPath := GetSocketPath()
	os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	s.listener = listener

	log.Infof("dsearch API Server listening on: %s", socketPath)
	log.Info("Protocol: JSON over Unix socket")
	log.Info("Request format: {\"id\": <any>, \"method\": \"...\", \"params\": {...}}")
	log.Info("Response format: {\"id\": <any>, \"result\": {...}} or {\"id\": <any>, \"error\": \"...\"}")

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return err
		}
		go s.handleConnection(conn)
	}
}

func (s *UnixServer) Close() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}
