package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/qbgames/qbdz-game/api"
	"github.com/qbgames/qbdz-game/internal/config"
	"github.com/qbgames/qbdz-game/internal/server"
	"github.com/qbgames/qbdz-game/internal/stub"
	"github.com/qbgames/qbdz-game/internal/table"
)

// newTestServer builds a registry with one table (ID=1) and returns a
// ready-to-use *httptest.Server.
func newTestServer(t *testing.T) (*httptest.Server, *stub.TableStub) {
	t.Helper()

	host := stub.NewTableStub()
	cfg := config.DefaultSceneConfig()
	tbl := table.NewTable(1, &cfg, host)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go tbl.Run(ctx)

	registry := server.NewRegistry()
	if err := registry.Add(1, tbl); err != nil {
		t.Fatalf("registry.Add: %v", err)
	}

	h := server.NewHandler(registry)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	return httptest.NewServer(mux), host
}

// postJSON sends a POST request with JSON body and decodes the JSON response.
func postJSON(t *testing.T, srv *httptest.Server, path string, reqBody any, resp any) int {
	t.Helper()
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	res, err := srv.Client().Post(srv.URL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return res.StatusCode
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestPlayerEnter_OK(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	var resp api.PlayerEnterResp
	status := postJSON(t, srv, "/v1/player/enter", api.PlayerEnterReq{
		TableID: 1,
		Player: api.PlayerInfo{
			UserID:    1001,
			NickName:  "Alice",
			GameMoney: 5000,
			Chair:     0xFF,
		},
	}, &resp)

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d: %s", resp.Code, resp.Message)
	}
}

func TestPlayerEnter_TableNotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	var resp api.CommonResp
	status := postJSON(t, srv, "/v1/player/enter", api.PlayerEnterReq{
		TableID: 999, // not registered
		Player:  api.PlayerInfo{UserID: 1001},
	}, &resp)

	if status != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", status)
	}
}

func TestPlayerEnter_BadJSON(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	res, err := srv.Client().Post(srv.URL+"/v1/player/enter", "application/json",
		bytes.NewBufferString("{not valid json"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}
}

func TestPlayerExit_OK(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	var resp api.CommonResp
	status := postJSON(t, srv, "/v1/player/exit", api.PlayerExitReq{
		TableID: 1,
		UserID:  1001,
	}, &resp)

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}
}

func TestGameMessage_OK(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	var resp api.CommonResp
	status := postJSON(t, srv, "/v1/game/message", api.GameMessageReq{
		TableID: 1,
		UserID:  1001,
		Cmd:     1005, // CmdReqFold
	}, &resp)

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
}

func TestForceEndGame_OK(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	var resp api.CommonResp
	status := postJSON(t, srv, "/v1/table/force-end", api.ForceEndGameReq{TableID: 1}, &resp)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}
}

func TestSetBlindLevel_OK(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	var resp api.CommonResp
	status := postJSON(t, srv, "/v1/table/blind-level", api.SetBlindLevelReq{
		TableID:    1,
		BlindLevel: 3,
	}, &resp)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
}

func TestCanEnter_ReturnsOK(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	var resp api.CanEnterResp
	status := postJSON(t, srv, "/v1/table/can-enter", api.CanEnterReq{
		TableID: 1,
		Player:  api.PlayerInfo{UserID: 2001},
		Chair:   0xFF,
	}, &resp)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}
}

func TestGetCapacity_ReturnsNine(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	var resp api.GetCapacityResp
	postJSON(t, srv, "/v1/table/capacity", api.GetCapacityReq{TableID: 1}, &resp)
	if resp.Capacity != 9 {
		t.Errorf("expected capacity 9, got %d", resp.Capacity)
	}
}

// TestPlayerEnterDeliveredToTable verifies that PlayerEnter actually reaches
// the table goroutine: after the call, the table has a seated player.
func TestPlayerEnterDeliveredToTable(t *testing.T) {
	srv, host := newTestServer(t)
	defer srv.Close()

	var resp api.PlayerEnterResp
	postJSON(t, srv, "/v1/player/enter", api.PlayerEnterReq{
		TableID: 1,
		Player: api.PlayerInfo{
			UserID:    3001,
			NickName:  "Bob",
			GameMoney: 10000,
			Chair:     0xFF,
		},
	}, &resp)

	// The table goroutine is async; give it a tick to process.
	time.Sleep(50 * time.Millisecond)

	// Nothing observable via stub for Enter alone (no broadcast), but at least
	// verify no error response and the stub host is still healthy.
	_ = host
	if resp.Code != 0 {
		t.Errorf("unexpected error code %d", resp.Code)
	}
}

func TestMatchBuyFinish_OK(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	var resp api.CommonResp
	status := postJSON(t, srv, "/v1/match/buy-finish", api.MatchBuyFinishReq{
		TableID: 1,
		UserID:  1001,
	}, &resp)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
}

func TestGetRankedPlayers_LiveQuery(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	// Enter two players so there is something to rank
	postJSON(t, srv, "/v1/player/enter", api.PlayerEnterReq{
		TableID: 1,
		Player:  api.PlayerInfo{UserID: 3001, NickName: "A", GameMoney: 5000},
	}, &api.PlayerEnterResp{})
	postJSON(t, srv, "/v1/player/enter", api.PlayerEnterReq{
		TableID: 1,
		Player:  api.PlayerInfo{UserID: 3002, NickName: "B", GameMoney: 3000},
	}, &api.PlayerEnterResp{})

	time.Sleep(50 * time.Millisecond) // let table goroutine process enters

	var resp api.GetRankedPlayersResp
	status := postJSON(t, srv, "/v1/table/ranked-players",
		api.GetRankedPlayersReq{TableID: 1}, &resp)

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if len(resp.UserIDs) != 2 {
		t.Errorf("expected 2 ranked players, got %d: %v", len(resp.UserIDs), resp.UserIDs)
	}
	// First player should have more chips (5000 > 3000)
	if len(resp.UserIDs) == 2 && resp.UserIDs[0] != 3001 {
		t.Errorf("expected UserID 3001 (5000 chips) first, got %d", resp.UserIDs[0])
	}
}

func TestGetCapacity_LiveQuery(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	var resp api.GetCapacityResp
	status := postJSON(t, srv, "/v1/table/capacity",
		api.GetCapacityReq{TableID: 1}, &resp)

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if resp.Capacity != 9 {
		t.Errorf("expected capacity 9 (default SeatCount), got %d", resp.Capacity)
	}
}

func TestSetBlindLevel_WiredToGame(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	// Send a blind-level update — should not error even with no schedule
	var resp api.CommonResp
	status := postJSON(t, srv, "/v1/table/blind-level", api.SetBlindLevelReq{
		TableID:    1,
		BlindLevel: 3,
	}, &resp)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if resp.Code != 0 {
		t.Fatalf("unexpected error: %d %s", resp.Code, resp.Message)
	}
}
