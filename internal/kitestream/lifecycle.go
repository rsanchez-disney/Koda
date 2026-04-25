package kitestream

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os/signal"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const DefaultPort = 7700

func pidPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".koda", "kitestream.pid")
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// IsRunning checks if a KiteStream server process is alive.
func IsRunning() bool {
	pid, err := ReadPID()
	if err != nil {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

// ReadPID reads the stored PID.
func ReadPID() (int, error) {
	data, err := os.ReadFile(pidPath())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func writePID(pid int) {
	os.MkdirAll(filepath.Dir(pidPath()), 0o755)
	os.WriteFile(pidPath(), []byte(strconv.Itoa(pid)), 0o644)
}

func removePID() {
	os.Remove(pidPath())
}

// Stop sends SIGTERM to the running KiteStream server.
func Stop() error {
	pid, err := ReadPID()
	if err != nil {
		return fmt.Errorf("kitestream is not running")
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		removePID()
		return fmt.Errorf("process %d not found", pid)
	}
	if err := p.Signal(syscall.SIGTERM); err != nil {
		removePID()
		return fmt.Errorf("failed to stop pid %d: %w", pid, err)
	}
	// Wait up to 5s for graceful shutdown
	for i := 0; i < 50; i++ {
		if p.Signal(syscall.Signal(0)) != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	removePID()
	return nil
}

// Launch starts the KiteStream server (Go-embedded HTTP server in a goroutine).
func Launch(steerRoot, targetDir string, port int) error {
	if IsRunning() {
		fmt.Printf("KiteStream already running (pid %d)\n", mustReadPID())
		openBrowser(port)
		return nil
	}

	if port == 0 {
		port = DefaultPort
	}

	// Check port is free
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("port %d is already in use — run 'lsof -ti:%d | xargs kill' or use --port", port, port)
	}
	ln.Close()

	token := generateToken()
	srv := NewServer(steerRoot, targetDir, port, token)

	// Start server in background goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	// Wait for health or error
	for i := 0; i < 30; i++ {
		select {
		case err := <-errCh:
			return fmt.Errorf("server failed: %w", err)
		default:
		}
		time.Sleep(100 * time.Millisecond)
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			goto ready
		}
	}
	return fmt.Errorf("kitestream failed to start within 3s")

ready:

	writePID(os.Getpid())
	fmt.Printf("✅ KiteStream running at http://localhost:%d\n", port)
	fmt.Println("Press Ctrl+C to stop")
	openBrowser(port)

	// Block until interrupt or server error
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sigCh:
		fmt.Println("\nShutting down...")
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
	}
	removePID()
	return nil
}

func openBrowser(port int) {
	url := fmt.Sprintf("http://localhost:%d", port)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}
	if cmd != nil {
		cmd.Start()
	}
}

func mustReadPID() int {
	pid, _ := ReadPID()
	return pid
}

// Status returns a human-readable status string.
func Status(port int) string {
	if !IsRunning() {
		return "stopped"
	}
	pid, _ := ReadPID()
	return fmt.Sprintf("running (pid %d, port %d)", pid, port)
}
