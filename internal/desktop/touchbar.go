package desktop

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

type TouchBarProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	mu     sync.Mutex
	active bool
}

var globalTBProcess *TouchBarProcess

// StartPhysicalTouchBarAgent launches the native Swift helper if running on macOS
func StartPhysicalTouchBarAgent(port int) *TouchBarProcess {
	if runtime.GOOS != "darwin" {
		return nil
	}

	// Locate touchbar_agent
	execPath, err := os.Executable()
	candidates := []string{
		"native/touchbar_agent",
		"./native/touchbar_agent",
		filepath.Join(filepath.Dir(execPath), "touchbar_agent"),
		filepath.Join(filepath.Dir(execPath), "..", "MacOS", "touchbar_agent"),
	}

	var binPath string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			binPath = c
			break
		}
	}

	if binPath == "" {
		return nil
	}

	cmd := exec.Command(binPath, "-port", fmt.Sprintf("%d", port))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil
	}

	stderr, err := cmd.StderrPipe()
	if err == nil {
		go func() {
			sc := bufio.NewScanner(stderr)
			for sc.Scan() {
				// Scan stderr for status
			}
		}()
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Could not start touchbar_agent: %v", err)
		return nil
	}

	p := &TouchBarProcess{
		cmd:   cmd,
		stdin: stdin,
	}
	globalTBProcess = p

	return p
}

func (p *TouchBarProcess) SetState(active bool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stdin == nil {
		return
	}

	if active {
		_, _ = io.WriteString(p.stdin, "show\n")
		p.active = true
	} else {
		_, _ = io.WriteString(p.stdin, "hide\n")
		p.active = false
	}
}

func (p *TouchBarProcess) Stop() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stdin != nil {
		_, _ = io.WriteString(p.stdin, "quit\n")
		_ = p.stdin.Close()
	}
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
}
