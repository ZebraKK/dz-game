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
	"github.com/qbgames/qbdz-game/internal/gatewayconfig"
	"github.com/qbgames/qbdz-game/internal/persistence"
	"github.com/qbgames/qbdz-game/internal/tableapp"
)

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	defaults := gatewayconfig.Default()
	var identityFlags stringList
	var allowedOrigins stringList
	configPath := flag.String("config", "", "YAML gateway config file")
	addr := flag.String("addr", defaults.Addr, "HTTP listen address")
	serverID := flag.String("server-id", defaults.ServerID, "authentication server ID")
	identityFile := flag.String("identity-file", "", "JSON identity registry file")
	historyFile := flag.String("history-file", "", "append-only JSONL hand history file")
	smallBlind := flag.Uint64("small-blind", defaults.Table.SmallBlind, "small blind")
	bigBlind := flag.Uint64("big-blind", defaults.Table.BigBlind, "big blind")
	startingStack := flag.Uint64("starting-stack", defaults.Table.StartingStack, "starting stack")
	actionTimeout := flag.Duration("action-timeout", time.Duration(defaults.Table.ActionTimeoutMs)*time.Millisecond, "action timeout")
	disconnectGrace := flag.Duration("disconnect-grace", time.Duration(defaults.Table.DisconnectGraceMs)*time.Millisecond, "disconnect grace")
	startCountdown := flag.Duration("start-countdown", time.Duration(defaults.Table.StartCountdownMs)*time.Millisecond, "pre-hand countdown")
	resultDisplay := flag.Duration("result-display", time.Duration(defaults.Table.ResultDisplayMs)*time.Millisecond, "settlement display duration")
	flag.Var(&identityFlags, "identity", "registered identity as playerId=displayName=base64url-public-key")
	flag.Var(&allowedOrigins, "allowed-origins", "allowed WebSocket Origin pattern; repeatable")
	flag.Parse()

	explicit := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })

	runtime := defaults
	if *configPath != "" {
		loaded, err := gatewayconfig.Load(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load gateway config: %v\n", err)
			os.Exit(2)
		}
		runtime = loaded
	}
	if explicit["addr"] {
		runtime.Addr = *addr
	}
	if explicit["server-id"] {
		runtime.ServerID = *serverID
	}
	if explicit["identity-file"] {
		runtime.IdentityFile = *identityFile
	}
	if explicit["history-file"] {
		runtime.HistoryFile = *historyFile
	}
	if explicit["small-blind"] {
		runtime.Table.SmallBlind = *smallBlind
	}
	if explicit["big-blind"] {
		runtime.Table.BigBlind = *bigBlind
	}
	if explicit["starting-stack"] {
		runtime.Table.StartingStack = *startingStack
	}
	if explicit["action-timeout"] {
		runtime.Table.ActionTimeoutMs = actionTimeout.Milliseconds()
	}
	if explicit["disconnect-grace"] {
		runtime.Table.DisconnectGraceMs = disconnectGrace.Milliseconds()
	}
	if explicit["start-countdown"] {
		runtime.Table.StartCountdownMs = startCountdown.Milliseconds()
	}
	if explicit["result-display"] {
		runtime.Table.ResultDisplayMs = resultDisplay.Milliseconds()
	}
	if explicit["allowed-origins"] {
		runtime.AllowedOrigins = append([]string(nil), allowedOrigins...)
	}
	if err := runtime.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "invalid gateway configuration: %v\n", err)
		os.Exit(2)
	}

	identities := make([]gateway.Identity, 0, len(identityFlags))
	if runtime.IdentityFile != "" {
		loaded, err := persistence.LoadIdentities(runtime.IdentityFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load identity file: %v\n", err)
			os.Exit(2)
		}
		identities = append(identities, loaded...)
	}
	for _, raw := range identityFlags {
		identity, err := parseIdentity(raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid identity %q: %v\n", raw, err)
			os.Exit(2)
		}
		identities = append(identities, identity)
	}
	if len(identities) == 0 {
		fmt.Fprintln(os.Stderr, "at least one -identity or -identity-file is required")
		os.Exit(2)
	}
	seenIDs := make(map[string]bool)
	for _, identity := range identities {
		if seenIDs[identity.PlayerID] {
			fmt.Fprintf(os.Stderr, "duplicate playerId %q\n", identity.PlayerID)
			os.Exit(2)
		}
		seenIDs[identity.PlayerID] = true
	}

	table := tableapp.NewTable(runtime.NewTableConfig())
	defer table.Close()
	if runtime.HistoryFile != "" {
		recorder, err := persistence.NewJSONLHistoryRecorder(runtime.HistoryFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "initialize history recorder: %v\n", err)
			os.Exit(2)
		}
		table.SetRecorder(recorder)
	}
	gatewayServer := gateway.NewServer(table, identities, runtime.ServerID, slog.Default())
	gatewayServer.SetAllowedOrigins(runtime.AllowedOrigins)

	mux := http.NewServeMux()
	mux.Handle("/ws", gatewayServer)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{
		Addr:              runtime.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		slog.Info("poker gateway listening",
			"addr", runtime.Addr,
			"identities", len(identities),
			"serverID", runtime.ServerID,
			"allowedOrigins", len(runtime.AllowedOrigins),
		)
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
