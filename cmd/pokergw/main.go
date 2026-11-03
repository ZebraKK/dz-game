package main

import (
	"context"
	"crypto/ed25519"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/qbgames/qbdz-game/internal/gateway"
	"github.com/qbgames/qbdz-game/internal/tableapp"
)

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	var identityFlags stringList
	addr := flag.String("addr", ":8080", "HTTP listen address")
	serverID := flag.String("server-id", "poker-local-1", "authentication server ID")
	smallBlind := flag.Uint64("small-blind", 10, "small blind")
	bigBlind := flag.Uint64("big-blind", 20, "big blind")
	startingStack := flag.Uint64("starting-stack", 2000, "starting stack")
	actionTimeout := flag.Duration("action-timeout", 90*time.Second, "action timeout")
	disconnectGrace := flag.Duration("disconnect-grace", 90*time.Second, "disconnect grace")
	startCountdown := flag.Duration("start-countdown", 3*time.Second, "pre-hand countdown")
	resultDisplay := flag.Duration("result-display", 5*time.Second, "settlement display duration")
	flag.Var(&identityFlags, "identity", "registered identity as playerId=displayName=base64url-public-key")
	flag.Parse()

	if *smallBlind == 0 || *bigBlind <= *smallBlind || *startingStack < *bigBlind {
		fmt.Fprintln(os.Stderr, "invalid table configuration")
		os.Exit(2)
	}
	if *actionTimeout <= 0 || *disconnectGrace <= 0 || *startCountdown < 0 || *resultDisplay <= 0 {
		fmt.Fprintln(os.Stderr, "invalid timing configuration")
		os.Exit(2)
	}

	if len(identityFlags) == 0 {
		fmt.Fprintln(os.Stderr, "at least one -identity is required")
		os.Exit(2)
	}

	identities := make([]gateway.Identity, 0, len(identityFlags))
	for _, raw := range identityFlags {
		identity, err := parseIdentity(raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid identity %q: %v\n", raw, err)
			os.Exit(2)
		}
		identities = append(identities, identity)
	}

	cfg := tableapp.DefaultConfig()
	cfg.SmallBlind = *smallBlind
	cfg.BigBlind = *bigBlind
	cfg.StartingStack = *startingStack
	cfg.ActionTimeout = *actionTimeout
	cfg.DisconnectGrace = *disconnectGrace
	cfg.StartCountdown = *startCountdown
	cfg.ResultDisplay = *resultDisplay
	table := tableapp.NewTable(cfg)
	defer table.Close()
	gatewayServer := gateway.NewServer(table, identities, *serverID, slog.Default())

	mux := http.NewServeMux()
	mux.Handle("/ws", gatewayServer)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		slog.Info("poker gateway listening", "addr", *addr, "identities", len(identities), "serverID", *serverID)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("gateway server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	slog.Info("poker gateway stopped")
}

func parseIdentity(raw string) (gateway.Identity, error) {
	parts := strings.SplitN(raw, "=", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return gateway.Identity{}, fmt.Errorf("expected playerId=displayName=publicKey")
	}
	publicKey, err := gateway.ParsePublicKey(parts[2])
	if err != nil {
		return gateway.Identity{}, err
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return gateway.Identity{}, fmt.Errorf("invalid public key length")
	}
	return gateway.NewIdentity(parts[0], parts[1], publicKey), nil
}
