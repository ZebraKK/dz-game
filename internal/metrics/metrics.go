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
)
