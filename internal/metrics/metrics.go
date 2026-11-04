// Package metrics exposes runtime counters for gamesvr via the standard
// expvar package (available at GET /debug/vars as JSON).
//
// No external dependencies — expvar ships with the Go standard library.
// Monitoring tools (Telegraf, Victoria Metrics, etc.) can scrape /debug/vars.
package metrics

import "expvar"

var (
	// ActiveTables is the current number of running table goroutines.
	ActiveTables = expvar.NewInt("gamesvr_active_tables")

	// HandsTotal counts completed Texas Hold'em hands across all tables.
	HandsTotal = expvar.NewInt("gamesvr_hands_total")

	// ActionsTotal counts processed player actions (fold/check/call/bet/raise/allin).
	ActionsTotal = expvar.NewInt("gamesvr_actions_total")

	// RobotActionsTotal counts actions taken by robot players.
	RobotActionsTotal = expvar.NewInt("gamesvr_robot_actions_total")

	// BillsTotal counts SetGameBill calls (financial settlements).
	BillsTotal = expvar.NewInt("gamesvr_bills_total")

	// HTTPRequests counts total HTTP requests handled.
	HTTPRequests = expvar.NewInt("gamesvr_http_requests_total")

	// PokerConnectionsTotal counts accepted player gateway WebSocket connections.
	PokerConnectionsTotal = expvar.NewInt("poker_connections_total")

	// PokerActiveConnections is the current number of accepted WebSocket connections.
	PokerActiveConnections = expvar.NewInt("poker_active_connections")

	// PokerAuthFailuresTotal counts rejected authentication attempts.
	PokerAuthFailuresTotal = expvar.NewInt("poker_auth_failures_total")

	// PokerMessagesTotal counts authenticated client messages read from sockets.
	PokerMessagesTotal = expvar.NewInt("poker_messages_total")

	// PokerRateLimitedTotal counts messages rejected by the per-connection limiter.
	PokerRateLimitedTotal = expvar.NewInt("poker_rate_limited_total")

	// PokerRateLimitClosesTotal counts connections closed for repeated rate limiting.
	PokerRateLimitClosesTotal = expvar.NewInt("poker_rate_limit_closes_total")

	// PokerHandsTotal counts started poker hands.
	PokerHandsTotal = expvar.NewInt("poker_hands_total")

	// PokerActionsTotal counts successfully applied poker actions, including automatic actions.
	PokerActionsTotal = expvar.NewInt("poker_actions_total")
)
