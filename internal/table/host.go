// Package table implements the poker table actor: one goroutine per table,
// all state mutation is single-threaded, no mutexes needed in game logic.
//
// The TableHost interface is the Go equivalent of C++ ITable / ISoTableManager.
// C++ reference: Source/Manager/SoITableManager.h
package table

// BillResult carries the financial outcome for one player after a hand.
type BillResult struct {
	UserID     uint32
	BillID     uint32 // GAME_BILL_USER=300, GAME_BILL_TABLE=301, etc.
	WinChips   int64  // positive=win, negative=loss
	BetChips   uint64
	BuyIn      uint64
	ServiceFee uint64
	BillData   string // serialized bill payload (matches C++ rstSOstr)
}

// PlayerInfo holds the basic profile data the table needs from the upper server.
type PlayerInfo struct {
	UserID    uint32
	NickName  string
	Avatar    string
	RoleID    uint32
	GameMoney uint64 // current coin balance
	Level     uint32
}

// TableHost is the outbound capability interface used by the table to
// communicate with external services (lobby, billing, networking).
//
// During development: satisfied by internal/stub.TableStub (in-memory).
// In production:      satisfied by a gRPC adapter calling Go peripheral services.
//
// All methods are called from inside the table goroutine, so implementations
// must handle their own concurrency. Use a timeout context on every call
// to avoid blocking the table loop indefinitely.
type TableHost interface {
	// ── Messaging ──────────────────────────────────────────────────────────

	// SendToAll broadcasts a Protobuf-serialized message to all seated players.
	// cmd is the command ID (matches C++ dwCMD values in DZGame.pb.h).
	SendToAll(cmd uint32, payload []byte) error

	// SendToPlayer sends a message to one specific player by userID.
	SendToPlayer(userID uint32, cmd uint32, payload []byte) error

	// SendExcept broadcasts to all players except one (e.g. hide hole cards).
	SendExcept(excludeUserID uint32, cmd uint32, payload []byte) error

	// ── Game lifecycle ──────────────────────────────────────────────────────

	// SetGameStart notifies the upper server that a hand has begun.
	SetGameStart() error

	// SetGameEnd notifies the upper server that a hand has ended.
	SetGameEnd() error

	// DismissGame asks the upper server to tear down this table.
	DismissGame() error

	// ── Finance ─────────────────────────────────────────────────────────────

	// SetGameBill reports the financial result for one player.
	// Equivalent to C++ SetGameBill(userID, billID, serializedStr).
	SetGameBill(bill BillResult) error

	// SetGameCoin adjusts a player's coin balance (positive=credit, negative=debit).
	SetGameCoin(userID uint32, delta int64, source uint32) error

	// SetGameExp awards experience points to a player.
	SetGameExp(userID uint32, delta int32, source uint32) error

	// ── Player management ───────────────────────────────────────────────────

	// KickPlayer removes a player from the table.
	// reasonID matches C++ KICK_REASONS constants.
	KickPlayer(userID uint32, reasonID uint32) error

	// MakePlayerStand forces a player to stand up (leave their seat).
	MakePlayerStand(userID uint32) error

	// GetPlayerInfo retrieves current profile data for a player.
	GetPlayerInfo(userID uint32) (PlayerInfo, error)

	// GetPlayerGameMoney returns a player's current coin balance.
	GetPlayerGameMoney(userID uint32) (uint64, error)

	// ── SNG / MTT ───────────────────────────────────────────────────────────

	// SetMatchChip adjusts a player's tournament chip count.
	SetMatchChip(userID uint32, delta int32, source uint32) error

	// SetMatchWeedOut marks a player as eliminated from the tournament.
	SetMatchWeedOut(userID uint32, eliminated bool, source uint32) error

	// ── Cowboy ──────────────────────────────────────────────────────────────

	// CowboyBet records a Cowboy bet placement.
	CowboyBet(userID uint32, money uint64) error

	// ── Queries ─────────────────────────────────────────────────────────────

	// GetSceneType returns the scene type constant (e.g. ST_CLASSIC=10301).
	GetSceneType() uint32

	// GetSceneLevel returns the scene level identifier.
	GetSceneLevel() uint32

	// IsMatch returns true if this table is part of a tournament.
	IsMatch() bool

	// IsUnderMaintenance returns true if the server is in maintenance mode.
	IsUnderMaintenance() bool

	// GetSysTime returns current Unix timestamp (seconds).
	GetSysTime() int64

	// GetTableID returns the unique table identifier.
	GetTableID() uint32

	// SendTaskEvent reports a task/achievement trigger for a player.
	SendTaskEvent(userID uint32, conditionID uint32) error

	// SendBroadcastEvent triggers a broadcast (jackpot, lucky dog, etc.).
	SendBroadcastEvent(userID uint32, ratio uint32, money uint32)

	// ReportJackpot reports a jackpot win.
	ReportJackpot(userIDs []uint32, cardType uint32)
}
