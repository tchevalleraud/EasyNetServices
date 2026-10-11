// Package supervisor controls the services supervised by s6 inside the container.
package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Supervisor starts, stops and inspects supervised services.
type Supervisor interface {
	// Available reports whether services can be controlled, which is not the
	// case when the server runs outside of the container.
	Available() bool
	Up(ctx context.Context, name string) error
	Down(ctx context.Context, name string) error
	Restart(ctx context.Context, name string) error
	IsUp(ctx context.Context, name string) (bool, error)
}

// S6 controls services through the s6 command-line tools.
type S6 struct {
	// ServiceDir is the s6 scan directory, /run/service with s6-overlay.
	ServiceDir string
}

// NewS6 returns a supervisor for the s6-overlay scan directory.
func NewS6() *S6 {
	return &S6{ServiceDir: "/run/service"}
}

// Available implements Supervisor.
func (s *S6) Available() bool {
	_, err := exec.LookPath("s6-svc")
	if err != nil {
		return false
	}
	info, err := os.Stat(s.ServiceDir)
	return err == nil && info.IsDir()
}

// Up implements Supervisor.
func (s *S6) Up(ctx context.Context, name string) error {
	return s.svc(ctx, name, "-u")
}

// Down implements Supervisor.
func (s *S6) Down(ctx context.Context, name string) error {
	return s.svc(ctx, name, "-d")
}

// Restart implements Supervisor. The service is brought up if it was down.
func (s *S6) Restart(ctx context.Context, name string) error {
	// -u marks the service as wanted up, -r sends SIGTERM so s6 restarts it.
	return s.svc(ctx, name, "-u", "-r")
}

// IsUp implements Supervisor.
func (s *S6) IsUp(ctx context.Context, name string) (bool, error) {
	out, err := exec.CommandContext(ctx, "s6-svstat", "-o", "up", s.path(name)).Output()
	if err != nil {
		return false, fmt.Errorf("s6-svstat %s: %w", name, err)
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

func (s *S6) svc(ctx context.Context, name string, flags ...string) error {
	args := append(flags, s.path(name))
	if out, err := exec.CommandContext(ctx, "s6-svc", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("s6-svc %s %s: %w: %s", strings.Join(flags, " "), name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *S6) path(name string) string {
	return filepath.Join(s.ServiceDir, name)
}
