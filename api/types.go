// Package api defines the JSON request and response types for the gamesvr HTTP service.
// These types replace the Protobuf-generated code and map 1:1 to the C++ IGame interface.
//
// All endpoints accept POST with Content-Type: application/json.
// Successful responses carry code=0; errors carry a non-zero code and a message.
package api

// ── Common ────────────────────────────────────────────────────────────────────

// CommonResp is returned by endpoints that have no data to return on success.
type CommonResp struct {
	Code    int32  `json:"code"`              // 0 = success
	Message string `json:"message,omitempty"` // human-readable error description
}

// PlayerInfo mirrors C++ COnlineGameData fields needed by game logic.
type PlayerInfo struct {
	UserID    uint32 `json:"user_id"`
	RoleID    uint32 `json:"role_id"`
	NickName  string `json:"nick_name"`
	Avatar    string `json:"avatar"`
	GameMoney uint64 `json:"game_money"` // current coin balance
	Level     uint32 `json:"level"`
	Chair     uint32 `json:"chair"` // requested seat; 0xFF = any
}

// ── Player lifecycle ──────────────────────────────────────────────────────────

// POST /v1/player/enter
// C++: OnPlayerEnter(COnlineGameData)
type PlayerEnterReq struct {
	TableID uint32     `json:"table_id"`
	Player  PlayerInfo `json:"player"`
}

type PlayerEnterResp struct {
	Code    int32  `json:"code"`
	Message string `json:"message,omitempty"`
	Status  int32  `json:"status"` // 0=Sit, 1=Stand
}

// POST /v1/player/exit
// C++: OnPlayerExit(userId)
type PlayerExitReq struct {
	TableID uint32 `json:"table_id"`
	UserID  uint32 `json:"user_id"`
}

// POST /v1/player/offline
// C++: OnPlayerOffline(userId)
type PlayerOfflineReq struct {
	TableID uint32 `json:"table_id"`
	UserID  uint32 `json:"user_id"`
}

// POST /v1/player/reconnect
// C++: OnGameUserReplay(userId)
type PlayerReconnectReq struct {
	TableID uint32 `json:"table_id"`
	UserID  uint32 `json:"user_id"`
}

// POST /v1/player/ready
// C++: OnGameUserReady(userId)
type PlayerReadyReq struct {
	TableID uint32 `json:"table_id"`
	UserID  uint32 `json:"user_id"`
}

// POST /v1/player/sit
// C++: OnGameUserSit(userId)
type PlayerSitReq struct {
	TableID uint32 `json:"table_id"`
	UserID  uint32 `json:"user_id"`
	Chair   uint32 `json:"chair"`
}

// POST /v1/player/stand
// C++: OnGameUserStand(userId)
type PlayerStandReq struct {
	TableID uint32 `json:"table_id"`
	UserID  uint32 `json:"user_id"`
}

// POST /v1/player/discard
// C++: OnPlayerDiscard(userId)
type PlayerDiscardReq struct {
	TableID uint32 `json:"table_id"`
	UserID  uint32 `json:"user_id"`
}

// ── Game messages ─────────────────────────────────────────────────────────────

// POST /v1/game/message
// C++: OnGameMsg(userId, chair, cmd, buf, len)
// Payload is the raw client protocol buffer encoded as base64 (standard encoding).
type GameMessageReq struct {
	TableID uint32 `json:"table_id"`
	UserID  uint32 `json:"user_id"`
	Chair   uint32 `json:"chair"`
	Cmd     uint32 `json:"cmd"`
	Payload []byte `json:"payload"` // encoding/json marshals []byte as base64
}

// ── Table queries ─────────────────────────────────────────────────────────────

// POST /v1/table/can-enter
// C++: CanEnter(player, status, chair)
type CanEnterReq struct {
	TableID uint32     `json:"table_id"`
	Player  PlayerInfo `json:"player"`
	Chair   uint32     `json:"chair"` // 0xFF = any
}

type CanEnterResp struct {
	Code    int32  `json:"code"`
	Message string `json:"message,omitempty"`
	Status  int32  `json:"status"` // 0=Sit, 1=Stand
}

// POST /v1/table/can-sit
// C++: CanSitDown(player)
type CanSitDownReq struct {
	TableID uint32     `json:"table_id"`
	Player  PlayerInfo `json:"player"`
}

type CanSitDownResp struct {
	Code    int32  `json:"code"`
	Message string `json:"message,omitempty"`
}

// POST /v1/table/can-change
// C++: CanChangeTable(userId)
type CanChangeTableReq struct {
	TableID uint32 `json:"table_id"`
	UserID  uint32 `json:"user_id"`
}

type CanChangeTableResp struct {
	Code    int32  `json:"code"`
	Message string `json:"message,omitempty"`
}

// POST /v1/table/ranked-players
// C++: GetRankedPlayers(vector<uint32_t>)
type GetRankedPlayersReq struct {
	TableID uint32 `json:"table_id"`
}

type GetRankedPlayersResp struct {
	UserIDs []uint32 `json:"user_ids"`
}

// POST /v1/table/capacity
// C++: GetCapacity()
type GetCapacityReq struct {
	TableID uint32 `json:"table_id"`
}

type GetCapacityResp struct {
	Capacity int32 `json:"capacity"`
}

// ── Table control ─────────────────────────────────────────────────────────────

// POST /v1/table/force-end
// C++: OnGameSetGameEnd()
type ForceEndGameReq struct {
	TableID uint32 `json:"table_id"`
}

// POST /v1/table/reload-config
// C++: ReloadCfg()
type ReloadConfigReq struct {
	TableID uint32 `json:"table_id"`
	CfgPath string `json:"cfg_path,omitempty"`
}

// POST /v1/table/blind-level
// C++: OnGameBlindLevel(blindLevel)
type SetBlindLevelReq struct {
	TableID    uint32 `json:"table_id"`
	BlindLevel uint32 `json:"blind_level"`
}

// POST /v1/table/friend-cfg
// C++: SetFriendTableCfg(roomType, anteType, exchangeIndex)
type SetFriendTableCfgReq struct {
	TableID       uint32 `json:"table_id"`
	RoomType      uint32 `json:"room_type"`
	AnteType      uint32 `json:"ante_type"`
	ExchangeIndex uint32 `json:"exchange_index"`
}

// POST /v1/game/launch
// C++: LaunchSOGame lifecycle hook
type LaunchGameReq struct {
	TableID    uint32 `json:"table_id"`
	SceneType  uint32 `json:"scene_type"`
	SceneLevel uint32 `json:"scene_level"`
}

// POST /v1/game/close
// C++: CloseSOGame lifecycle hook
type CloseGameReq struct {
	TableID uint32 `json:"table_id"`
}

// ── Match / tournament ────────────────────────────────────────────────────────

// POST /v1/match/wait-buy
// C++: OnMatchWaitBuy(userId, endTime)
type MatchWaitBuyReq struct {
	TableID    uint32 `json:"table_id"`
	UserID     uint32 `json:"user_id"`
	BuyEndTime uint32 `json:"buy_end_time"` // Unix timestamp
}

// POST /v1/match/buy-finish
// C++: OnMatchBuyFinish(userId)
type MatchBuyFinishReq struct {
	TableID uint32 `json:"table_id"`
	UserID  uint32 `json:"user_id"`
}

// POST /v1/jackpot/lucky-dogs
// C++: LuckyDogsBonusNotify(userId, bonus, totalMoney)
type LuckyDogsBonusReq struct {
	TableID      uint32 `json:"table_id"`
	UserID       uint32 `json:"user_id"`
	JackpotBonus uint32 `json:"jackpot_bonus"`
	TotalMoney   uint64 `json:"total_money"`
}
