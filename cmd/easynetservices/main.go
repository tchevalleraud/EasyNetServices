// Command easynetservices runs the EasyNetServices management server.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	// Embed the time zone database: the image does not ship tzdata.
	_ "time/tzdata"

	"github.com/tchevalleraud/EasyNetServices/internal/api"
	"github.com/tchevalleraud/EasyNetServices/internal/auth"
	"github.com/tchevalleraud/EasyNetServices/internal/config"
	"github.com/tchevalleraud/EasyNetServices/internal/ntp"
	"github.com/tchevalleraud/EasyNetServices/internal/store"
	"github.com/tchevalleraud/EasyNetServices/internal/supervisor"
	"github.com/tchevalleraud/EasyNetServices/internal/system"
	"github.com/tchevalleraud/EasyNetServices/internal/tlsutil"
	"github.com/tchevalleraud/EasyNetServices/internal/version"
	"github.com/tchevalleraud/EasyNetServices/web"
)

const usage = `Usage: easynetservices <command>

Commands:
  serve                     Run the server (default)
  healthcheck               Exit with status 0 when the local server is healthy
  reset-password <username> Set a temporary password on a local user
  version                   Print version information
`

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	var err error
	switch cmd {
	case "serve":
		err = serve()
	case "healthcheck":
		err = healthcheck()
	case "reset-password":
		if len(os.Args) != 3 {
			fmt.Fprint(os.Stderr, usage)
			os.Exit(2)
		}
		err = resetPassword(os.Args[2])
	case "version":
		fmt.Printf("easynetservices %s (commit %s, built %s)\n", version.Version, version.Commit, version.Date)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func serve() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.LogLevel,
		// Log timestamps in the configured time zone.
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && len(groups) == 0 {
				a.Value = slog.TimeValue(a.Value.Time().In(system.Location()))
			}
			return a
		},
	}))
	log.Info("starting EasyNetServices", "version", version.Version, "role", cfg.Role)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	st, err := store.Open(ctx, cfg.DataDir)
	if err != nil {
		return err
	}
	defer st.Close()

	authSvc, err := newAuthService(cfg, st)
	if err != nil {
		return err
	}
	created, err := authSvc.EnsureDefaultAdmin(ctx)
	if err != nil {
		return fmt.Errorf("create default admin: %w", err)
	}
	if created {
		log.Warn("default account created, the password must be changed at first login",
			"username", auth.DefaultAdminUsername, "password", auth.DefaultAdminPassword)
	}

	sysMgr := system.NewManager(st, log)
	if err := sysMgr.Init(ctx); err != nil {
		log.Error("failed to apply system settings", "error", err)
	}

	sup := supervisor.NewS6()
	ntpMgr := ntp.NewManager(st, sup, cfg.DataDir, log.With("service", "ntp"))
	// A service that fails to start must not prevent access to the interface,
	// where the configuration can be fixed.
	if err := ntpMgr.Init(ctx); err != nil {
		log.Error("failed to apply NTP configuration", "error", err)
	}

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           api.NewServer(cfg, st, authSvc, api.Services{System: sysMgr, NTP: ntpMgr}, web.FS(), log).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	certFile, keyFile, err := tlsFiles(cfg)
	if err != nil {
		return err
	}

	go purgeSessions(ctx, authSvc, log)

	errCh := make(chan error, 1)
	go func() {
		log.Info("web interface listening", "address", cfg.Listen, "tls", cfg.TLSMode)
		if certFile != "" {
			srv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
			errCh <- srv.ListenAndServeTLS(certFile, keyFile)
		} else {
			errCh <- srv.ListenAndServe()
		}
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
	}
	return nil
}

func newAuthService(cfg *config.Config, st *store.Store) (*auth.Service, error) {
	local, err := auth.NewLocalProvider(st)
	if err != nil {
		return nil, err
	}
	return auth.NewService(st, cfg.SessionTTL, local), nil
}

// tlsFiles returns the certificate and key to use, or empty strings when TLS is off.
func tlsFiles(cfg *config.Config) (string, string, error) {
	switch cfg.TLSMode {
	case config.TLSCustom:
		return cfg.TLSCert, cfg.TLSKey, nil
	case config.TLSSelfSigned:
		return tlsutil.EnsureSelfSigned(filepath.Join(cfg.DataDir, "tls"))
	default:
		return "", "", nil
	}
}

func purgeSessions(ctx context.Context, authSvc *auth.Service, log *slog.Logger) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := authSvc.PurgeExpiredSessions(ctx); err != nil {
				log.Warn("failed to purge expired sessions", "error", err)
			}
		}
	}
}

func healthcheck() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	_, port, err := net.SplitHostPort(cfg.Listen)
	if err != nil {
		return fmt.Errorf("invalid ENS_LISTEN: %w", err)
	}
	scheme := "https"
	if cfg.TLSMode == config.TLSOff {
		scheme = "http"
	}
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			// The check targets the local server, whose certificate may be self-signed.
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Get(fmt.Sprintf("%s://127.0.0.1:%s/healthz", scheme, port))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: HTTP %d", resp.StatusCode)
	}
	return nil
}

func resetPassword(username string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DataDir)
	if err != nil {
		return err
	}
	defer st.Close()

	authSvc, err := newAuthService(cfg, st)
	if err != nil {
		return err
	}
	temp, err := authSvc.ResetPassword(ctx, username)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("user %q not found", username)
	}
	if err != nil {
		return err
	}
	fmt.Printf("Temporary password for %s: %s\nIt must be changed at next login.\n", username, temp)
	return nil
}
