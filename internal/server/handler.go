package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/qbgames/qbdz-game/api"
	"github.com/qbgames/qbdz-game/internal/table"
)

// Handler holds the registry and implements all HTTP endpoints.
type Handler struct {
	registry *Registry
}

// NewHandler creates a Handler backed by the given registry.
func NewHandler(r *Registry) *Handler {
	return &Handler{registry: r}
}

// RegisterRoutes attaches all endpoints to mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Player lifecycle
	mux.HandleFunc("POST /v1/player/enter", h.playerEnter)
	mux.HandleFunc("POST /v1/player/exit", h.playerExit)
	mux.HandleFunc("POST /v1/player/offline", h.playerOffline)
	mux.HandleFunc("POST /v1/player/reconnect", h.playerReconnect)
	mux.HandleFunc("POST /v1/player/ready", h.playerReady)
	mux.HandleFunc("POST /v1/player/sit", h.playerSit)
	mux.HandleFunc("POST /v1/player/stand", h.playerStand)
	mux.HandleFunc("POST /v1/player/discard", h.playerDiscard)

	// Game messages
	mux.HandleFunc("POST /v1/game/message", h.gameMessage)
	mux.HandleFunc("POST /v1/game/launch", h.gameLaunch)
	mux.HandleFunc("POST /v1/game/close", h.gameClose)

	// Table queries (synchronous stubs; full logic wired in Phase 5)
	mux.HandleFunc("POST /v1/table/can-enter", h.canEnter)
	mux.HandleFunc("POST /v1/table/can-sit", h.canSitDown)
	mux.HandleFunc("POST /v1/table/can-change", h.canChangeTable)
	mux.HandleFunc("POST /v1/table/ranked-players", h.getRankedPlayers)
	mux.HandleFunc("POST /v1/table/capacity", h.getCapacity)

	// Table control
	mux.HandleFunc("POST /v1/table/force-end", h.forceEndGame)
	mux.HandleFunc("POST /v1/table/reload-config", h.reloadConfig)
	mux.HandleFunc("POST /v1/table/blind-level", h.setBlindLevel)
	mux.HandleFunc("POST /v1/table/friend-cfg", h.setFriendTableCfg)

	// Match / tournament
	mux.HandleFunc("POST /v1/match/wait-buy", h.matchWaitBuy)
	mux.HandleFunc("POST /v1/match/buy-finish", h.matchBuyFinish)

	// Jackpot
	mux.HandleFunc("POST /v1/jackpot/lucky-dogs", h.luckyDogsBonus)
}

// ── Player lifecycle ──────────────────────────────────────────────────────────

func (h *Handler) playerEnter(w http.ResponseWriter, r *http.Request) {
	var req api.PlayerEnterReq
	if !decode(w, r, &req) {
		return
	}
	tbl, ok := h.lookupTable(w, req.TableID)
	if !ok {
		return
	}
	msg := table.Msg{
		Kind:     table.MsgKindPlayerEnter,
		UserID:   req.Player.UserID,
		RoleID:   req.Player.RoleID,
		NickName: req.Player.NickName,
		Avatar:   req.Player.Avatar,
		ChipDesk: req.Player.GameMoney,
		Chair:    uint8(req.Player.Chair),
	}
	if !tbl.Send(msg) {
		writeError(w, http.StatusServiceUnavailable, "table inbox full")
		return
	}
	writeJSON(w, api.PlayerEnterResp{Code: 0, Status: 1}) // 1=Stand until seated
}

func (h *Handler) playerExit(w http.ResponseWriter, r *http.Request) {
	var req api.PlayerExitReq
	if !decode(w, r, &req) {
		return
	}
	if !h.sendSimple(w, req.TableID, table.Msg{
		Kind:   table.MsgKindPlayerExit,
		UserID: req.UserID,
	}) {
		return
	}
	writeJSON(w, api.CommonResp{Code: 0})
}

func (h *Handler) playerOffline(w http.ResponseWriter, r *http.Request) {
	var req api.PlayerOfflineReq
	if !decode(w, r, &req) {
		return
	}
	if !h.sendSimple(w, req.TableID, table.Msg{
		Kind:   table.MsgKindPlayerOffline,
		UserID: req.UserID,
	}) {
		return
	}
	writeJSON(w, api.CommonResp{Code: 0})
}

func (h *Handler) playerReconnect(w http.ResponseWriter, r *http.Request) {
	var req api.PlayerReconnectReq
	if !decode(w, r, &req) {
		return
	}
	if !h.sendSimple(w, req.TableID, table.Msg{
		Kind:   table.MsgKindPlayerReconnect,
		UserID: req.UserID,
	}) {
		return
	}
	writeJSON(w, api.CommonResp{Code: 0})
}

func (h *Handler) playerReady(w http.ResponseWriter, r *http.Request) {
	var req api.PlayerReadyReq
	if !decode(w, r, &req) {
		return
	}
	if !h.sendSimple(w, req.TableID, table.Msg{
		Kind:   table.MsgKindPlayerReady,
		UserID: req.UserID,
	}) {
		return
	}
	writeJSON(w, api.CommonResp{Code: 0})
}

func (h *Handler) playerSit(w http.ResponseWriter, r *http.Request) {
	var req api.PlayerSitReq
	if !decode(w, r, &req) {
		return
	}
	if !h.sendSimple(w, req.TableID, table.Msg{
		Kind:   table.MsgKindPlayerSit,
		UserID: req.UserID,
		Chair:  uint8(req.Chair),
	}) {
		return
	}
	writeJSON(w, api.CommonResp{Code: 0})
}

func (h *Handler) playerStand(w http.ResponseWriter, r *http.Request) {
	var req api.PlayerStandReq
	if !decode(w, r, &req) {
		return
	}
	if !h.sendSimple(w, req.TableID, table.Msg{
		Kind:   table.MsgKindPlayerStand,
		UserID: req.UserID,
	}) {
		return
	}
	writeJSON(w, api.CommonResp{Code: 0})
}

func (h *Handler) playerDiscard(w http.ResponseWriter, r *http.Request) {
	var req api.PlayerDiscardReq
	if !decode(w, r, &req) {
		return
	}
	if !h.sendSimple(w, req.TableID, table.Msg{
		Kind:   table.MsgKindPlayerDiscard,
		UserID: req.UserID,
	}) {
		return
	}
	writeJSON(w, api.CommonResp{Code: 0})
}

// ── Game messages ─────────────────────────────────────────────────────────────

func (h *Handler) gameMessage(w http.ResponseWriter, r *http.Request) {
	var req api.GameMessageReq
	if !decode(w, r, &req) {
		return
	}
	if !h.sendSimple(w, req.TableID, table.Msg{
		Kind:    table.MsgKindGameMessage,
		UserID:  req.UserID,
		Chair:   uint8(req.Chair),
		Cmd:     req.Cmd,
		Payload: req.Payload,
	}) {
		return
	}
	writeJSON(w, api.CommonResp{Code: 0})
}

func (h *Handler) gameLaunch(w http.ResponseWriter, r *http.Request) {
	// LaunchSOGame is a lifecycle hook; acknowledged immediately.
	var req api.LaunchGameReq
	if !decode(w, r, &req) {
		return
	}
	writeJSON(w, api.CommonResp{Code: 0})
}

func (h *Handler) gameClose(w http.ResponseWriter, r *http.Request) {
	// CloseSOGame is a lifecycle hook; acknowledged immediately.
	var req api.CloseGameReq
	if !decode(w, r, &req) {
		return
	}
	writeJSON(w, api.CommonResp{Code: 0})
}

// ── Table queries (synchronous stubs) ────────────────────────────────────────
// Phase 5 will wire these to the table goroutine via a reply-channel Msg.

func (h *Handler) canEnter(w http.ResponseWriter, r *http.Request) {
	var req api.CanEnterReq
	if !decode(w, r, &req) {
		return
	}
	// TODO Phase 5: query table goroutine for admission decision
	writeJSON(w, api.CanEnterResp{Code: 0, Status: 1}) // 1=Stand
}

func (h *Handler) canSitDown(w http.ResponseWriter, r *http.Request) {
	var req api.CanSitDownReq
	if !decode(w, r, &req) {
		return
	}
	writeJSON(w, api.CanSitDownResp{Code: 0})
}

func (h *Handler) canChangeTable(w http.ResponseWriter, r *http.Request) {
	var req api.CanChangeTableReq
	if !decode(w, r, &req) {
		return
	}
	writeJSON(w, api.CanChangeTableResp{Code: 0})
}

func (h *Handler) getRankedPlayers(w http.ResponseWriter, r *http.Request) {
	var req api.GetRankedPlayersReq
	if !decode(w, r, &req) {
		return
	}
	tbl, ok := h.lookupTable(w, req.TableID)
	if !ok {
		return
	}
	result := tbl.Query(table.MsgKindGetRankedPlayers, 2*time.Second)
	ids, _ := result.([]uint32)
	if ids == nil {
		ids = []uint32{}
	}
	writeJSON(w, api.GetRankedPlayersResp{UserIDs: ids})
}

func (h *Handler) getCapacity(w http.ResponseWriter, r *http.Request) {
	var req api.GetCapacityReq
	if !decode(w, r, &req) {
		return
	}
	tbl, ok := h.lookupTable(w, req.TableID)
	if !ok {
		return
	}
	result := tbl.Query(table.MsgKindGetCapacity, 2*time.Second)
	capacity, _ := result.(int32)
	writeJSON(w, api.GetCapacityResp{Capacity: capacity})
}

// ── Table control ─────────────────────────────────────────────────────────────

func (h *Handler) forceEndGame(w http.ResponseWriter, r *http.Request) {
	var req api.ForceEndGameReq
	if !decode(w, r, &req) {
		return
	}
	if !h.sendSimple(w, req.TableID, table.Msg{Kind: table.MsgKindForceEnd}) {
		return
	}
	writeJSON(w, api.CommonResp{Code: 0})
}

func (h *Handler) reloadConfig(w http.ResponseWriter, r *http.Request) {
	var req api.ReloadConfigReq
	if !decode(w, r, &req) {
		return
	}
	if !h.sendSimple(w, req.TableID, table.Msg{Kind: table.MsgKindReloadConfig}) {
		return
	}
	writeJSON(w, api.CommonResp{Code: 0})
}

func (h *Handler) setBlindLevel(w http.ResponseWriter, r *http.Request) {
	var req api.SetBlindLevelReq
	if !decode(w, r, &req) {
		return
	}
	if !h.sendSimple(w, req.TableID, table.Msg{
		Kind:       table.MsgKindSetBlindLevel,
		BlindLevel: req.BlindLevel,
	}) {
		return
	}
	writeJSON(w, api.CommonResp{Code: 0})
}

func (h *Handler) setFriendTableCfg(w http.ResponseWriter, r *http.Request) {
	var req api.SetFriendTableCfgReq
	if !decode(w, r, &req) {
		return
	}
	// TODO Phase 5: route to table via dedicated Msg kind
	writeJSON(w, api.CommonResp{Code: 0})
}

// ── Match / tournament ────────────────────────────────────────────────────────

func (h *Handler) matchWaitBuy(w http.ResponseWriter, r *http.Request) {
	var req api.MatchWaitBuyReq
	if !decode(w, r, &req) {
		return
	}
	// TODO Phase 5: notify table of rebuy window
	writeJSON(w, api.CommonResp{Code: 0})
}

func (h *Handler) matchBuyFinish(w http.ResponseWriter, r *http.Request) {
	var req api.MatchBuyFinishReq
	if !decode(w, r, &req) {
		return
	}
	if !h.sendSimple(w, req.TableID, table.Msg{
		Kind:   table.MsgKindBuyFinish,
		UserID: req.UserID,
	}) {
		return
	}
	writeJSON(w, api.CommonResp{Code: 0})
}

// ── Jackpot ───────────────────────────────────────────────────────────────────

func (h *Handler) luckyDogsBonus(w http.ResponseWriter, r *http.Request) {
	var req api.LuckyDogsBonusReq
	if !decode(w, r, &req) {
		return
	}
	// TODO Phase 5: notify table of jackpot award
	writeJSON(w, api.CommonResp{Code: 0})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// lookupTable finds a registered table or writes a 404 and returns false.
func (h *Handler) lookupTable(w http.ResponseWriter, tableID uint32) (*table.Table, bool) {
	tbl, ok := h.registry.Get(tableID)
	if !ok {
		writeError(w, http.StatusNotFound, "table not found")
		return nil, false
	}
	return tbl, true
}

// sendSimple looks up the table and delivers msg; writes error on failure.
func (h *Handler) sendSimple(w http.ResponseWriter, tableID uint32, msg table.Msg) bool {
	tbl, ok := h.lookupTable(w, tableID)
	if !ok {
		return false
	}
	if !tbl.Send(msg) {
		writeError(w, http.StatusServiceUnavailable, "table inbox full")
		return false
	}
	return true
}

// decode reads the JSON body into v. Writes a 400 and returns false on error.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return false
	}
	return true
}

// writeJSON serialises v as JSON with status 200.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response with the given HTTP status.
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.CommonResp{Code: int32(status), Message: msg})
}
