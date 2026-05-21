package mihomo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"
)

type Process struct {
	mu        sync.Mutex
	bin       string
	configDir string
	controller string
	secret    string
	client    *Client
	cmd       *exec.Cmd
	started   bool
}

func NewProcess(bin, configDir, controller, secret string) *Process {
	return &Process{
		bin:        bin,
		configDir:  configDir,
		controller: controller,
		secret:     secret,
		client:     NewClient(controller, secret),
	}
}

func (p *Process) EnsureRunning(ctx context.Context, configRaw []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	patched, err := PatchConfig(configRaw, p.controller, p.secret)
	if err != nil {
		return err
	}
	configPath, err := WriteConfig(p.configDir, patched)
	if err != nil {
		return err
	}

	if p.started && p.cmd != nil && p.cmd.Process != nil {
		if err := p.client.ReloadConfig(ctx, configPath); err != nil {
			log.Printf("mihomo reload failed, restarting: %v", err)
			_ = p.stopLocked()
		} else {
			return nil
		}
	}

	if err := p.startLocked(ctx); err != nil {
		return err
	}
	return p.waitReady(ctx)
}

func (p *Process) startLocked(ctx context.Context) error {
	if _, err := os.Stat(p.bin); err != nil {
		return fmt.Errorf("mihomo binary %s: %w", p.bin, err)
	}
	p.cmd = exec.Command(p.bin, "-d", p.configDir)
	p.cmd.Stdout = os.Stdout
	p.cmd.Stderr = os.Stderr
	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("start mihomo: %w", err)
	}
	p.started = true
	log.Printf("mihomo started pid=%d dir=%s", p.cmd.Process.Pid, p.configDir)
	return nil
}

func (p *Process) waitReady(ctx context.Context) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		deadline, _ = ctx.Deadline()
	}
	for {
		if err := p.client.Ping(ctx); err == nil {
			return nil
		}
		if !p.processAlive() {
			return errors.New("mihomo exited (CPU may lack x86-64-v3; use compatible binary in image)")
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("mihomo controller not ready at %s", p.controller)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (p *Process) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stopLocked()
}

func (p *Process) stopLocked() error {
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
		_, _ = p.cmd.Process.Wait()
	}
	p.cmd = nil
	p.started = false
	return nil
}

func (p *Process) Client() *Client {
	return p.client
}

func (p *Process) processAlive() bool {
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	switch runtime.GOOS {
	case "linux", "darwin", "freebsd":
		return p.cmd.Process.Signal(syscall.Signal(0)) == nil
	default:
		return true
	}
}
