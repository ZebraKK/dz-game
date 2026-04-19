// Package player defines the player and watcher data models, and all
// player-related enums. All numeric values match the C++ enums in
// SoGameCommon.h exactly, to preserve wire protocol compatibility.
//
// C++ reference: Source/SoGameLogic/SoGameCommon.h, SoUser.h
package player

// NetStatus represents the network connection state of a player.
// Matches C++ UserNetStatus enum.
type NetStatus int

const (
	NetOnline  NetStatus = 0 // eumUserOnline
	NetOffline NetStatus = 1 // eumUserOffline
	NetExit    NetStatus = 2 // eumUserExit
)

// StatType represents the player's seat state visible to the table.
// Matches C++ PlayerStatType enum. Note: not all values are used.
type StatType int

const (
	StatGetOut  StatType = 1 // PST_GET_OUT  离开
	StatSit     StatType = 3 // PST_SIT      坐下未开始
	StatPlaying StatType = 5 // PST_PLAYING  正在玩
	StatStand   StatType = 8 // PST_STAND    未上桌（旁观）
)

// Action represents the betting action a player takes during a hand.
// Matches C++ eumPlayerAction enum.
type Action int

const (
	ActionNull  Action = 0 // EPA_NULL  初始化
	ActionCheck Action = 1 // EPA_CHECK 过牌
	ActionBet   Action = 2 // EPA_BET   下注
	ActionCall  Action = 3 // EPA_CALL  跟注
	ActionRaise Action = 4 // EPA_RAISE 加注
	ActionFold  Action = 5 // EPA_FOLD  弃牌
	ActionAllIn Action = 6 // EPA_ALLIN 全押
)

func (a Action) String() string {
	names := []string{"Null", "Check", "Bet", "Call", "Raise", "Fold", "AllIn"}
	if int(a) < len(names) {
		return names[a]
	}
	return "Unknown"
}

// SeatInfo describes a seat's special role in the current hand.
// Matches C++ eumSoTableSeatInfo enum.
type SeatInfo int

const (
	SeatNone       SeatInfo = 0 // 普通座位
	SeatDealer     SeatInfo = 1 // 庄家（Dealer Button）
	SeatBigBlind   SeatInfo = 2 // 大盲
	SeatSmallBlind SeatInfo = 3 // 小盲
)

// Player holds all per-seat state for one player at the table.
// Maps to C++ CSoPlayer in SoUser.h.
type Player struct {
	UserID   uint32
	NickName string
	Avatar   string
	RoleID   uint32

	// Network state
	NetStatus NetStatus

	// Seat state
	IsReady   bool // 已坐下并准备好
	InGame    bool // 参与当前这局
	IsDiscard bool // 标记为待移除

	// Betting state (reset each hand)
	Action       Action
	LastAction   Action
	ActionIsAuto bool // 动作是服务器自动生成的
	AutoAction   bool // 玩家开启了自动操作

	// Chips
	ChipDesk          uint64 // 桌上筹码（带入桌的）
	ChipBetThisStreet uint64 // 本下注轮已投入
	Ante              uint64 // 已下的 ante

	// SNG / rebuy
	SNGRank         uint32
	RebuyAfterGame  bool
	AddonAfterGame  bool
	IsInRebuyWindow bool

	// Leave-seat / sit-out logic
	LeaveSeat             bool
	WaitForBB             bool   // 等待轮到大盲再入局
	BBSeatWhenLeftSeat    uint8  // 离座时的大盲位置
	MissedBlindDebt       uint32 // 缺少的盲注债务

	// Statistics (cumulative this session)
	BuyChipTotal       uint64
	WinLoseChipTotal   int64  // 正=赢 负=输
	ChipBetThisHand    uint64
	ChipWonThisHand    uint64

	// Offline tracking
	OfflineTime uint32 // Unix timestamp when went offline

	// Timeout tracking
	ActionTimeoutCount uint8  // 本次就座期间超时次数
	ActionTimedOut     bool   // 当前行动已超时

	// Training mode
	TrainLeaveHintTime uint32

	// Settlement flag
	BeenSettled bool

	// Robot marker — if true, the table actor makes decisions automatically.
	IsRobot bool
}

// NewPlayer creates a Player with sensible zero values.
func NewPlayer(userID uint32, nick, avatar string, chipDesk uint64) *Player {
	return &Player{
		UserID:    userID,
		NickName:  nick,
		Avatar:    avatar,
		ChipDesk:  chipDesk,
		NetStatus: NetOnline,
	}
}

// IsActive returns true if the player is participating in the current hand
// and has not folded or gone all-in (still has decisions to make).
func (p *Player) IsActive() bool {
	return p.InGame && p.Action != ActionFold
}

// IsAllIn returns true if the player has committed all chips this hand.
func (p *Player) IsAllIn() bool {
	return p.InGame && p.Action == ActionAllIn
}

// Watcher holds the state for a table observer (not playing).
// Maps to C++ CSoWatcher in SoUser.h.
type Watcher struct {
	UserID      uint32
	Chair       uint8  // which seat they are watching from
	RoleID      uint32
	NickName    string
	Avatar      string
	WatchIndex  uint32
}
