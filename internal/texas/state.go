// Package texas implements the Texas Hold'em poker game state machine.
// All game logic is pure (no I/O, no goroutines). The Table actor in
// internal/table/ calls these methods and processes the returned Events.
//
// C++ reference: Source/SoGameLogic/SoGameLogic.h/.cpp, SoGameCommon.h
package texas

// GameStatus represents the current phase of a Texas Hold'em hand.
// Values match C++ eumSoGameStatus in SoGameCommon.h exactly.
type GameStatus int

const (
	StatusNoStart      GameStatus = 0 // eumSoGsNoStart      等待开始
	StatusCountdown    GameStatus = 1 // eumSoGsCountDownStart 倒计时
	StatusPreFlop      GameStatus = 2 // eumSoGsPreFlop      翻牌前
	StatusFlop         GameStatus = 3 // eumSoGsFlop         翻牌
	StatusTurn         GameStatus = 4 // eumSoGsTurn         转牌
	StatusRiver        GameStatus = 5 // eumSoGsRiver        河牌
	StatusResult       GameStatus = 6 // eumSoGsResult       结算
)

func (s GameStatus) String() string {
	names := []string{
		"NoStart", "Countdown", "PreFlop", "Flop", "Turn", "River", "Result",
	}
	if int(s) < len(names) {
		return names[s]
	}
	return "Unknown"
}

// TableStatus represents whether the table itself is active.
// Matches C++ eumSoTableStatus.
type TableStatus int

const (
	TableWaiting  TableStatus = 0 // enumSoTsWaiting 等待中
	TableGaming   TableStatus = 1 // enumSoTsGaming  游戏中
	TableDismissed TableStatus = 2 // enumSoTsDismiss 已解散
)

// ResultReason describes why a hand ended.
// Matches C++ GameResultReason.
type ResultReason int

const (
	ReasonNull   ResultReason = -1 // GRR_NULL   无效
	ReasonNormal ResultReason = 0  // GRR_NORMAL 正常结束
	ReasonAllIn  ResultReason = 1  // GRR_ALLIN  AllIn结束
	ReasonFold   ResultReason = 2  // GRR_FOLD   弃牌结束
)

// TimerID identifies a game timer within a table.
// Defined here for use in Events; the table package manages the actual timers.
type TimerID int

const (
	TimerNone      TimerID = 0
	TimerAction    TimerID = 1 // 当前玩家行动超时
	TimerCountdown TimerID = 2 // 开局倒计时
	TimerResult    TimerID = 3 // 结算展示
	TimerRebuy     TimerID = 4 // Rebuy 等待
	TimerOffline   TimerID = 5 // 离线超时
)

// EventKind describes what action the Table actor should take.
type EventKind int

const (
	EventSendAll     EventKind = iota // 广播消息给桌上所有人
	EventSendTo                       // 发消息给指定玩家
	EventSendExcept                   // 广播但排除某玩家
	EventSetTimer                     // 设置定时器
	EventCancelTimer                  // 取消定时器
	EventSettleBill                   // 财务结算
	EventGameStart                    // 通知桌子游戏已开始
	EventGameEnd                      // 通知桌子游戏已结束
	EventKickPlayer                   // 踢出玩家
	EventUpdateSeat                   // 座位状态变化（通知外部）
)

// Event is the return value from all game logic methods.
// The table actor processes each event after calling a Game method.
type Event struct {
	Kind          EventKind
	TargetUserID  uint32        // for SendTo / KickPlayer
	ExcludeUserID uint32        // for SendExcept
	Cmd           uint32        // Protobuf command ID
	Payload       []byte        // serialized Protobuf message
	TimerID       TimerID       // for SetTimer / CancelTimer
	TimerDuration int64         // milliseconds, for SetTimer
	WinChips      int64         // for SettleBill: net win (after fee deducted)
	BetChips      uint64        // for SettleBill: total chips wagered this hand
	ServiceFee    uint64        // for SettleBill: rake charged to winner
	BillUserID    uint32        // for SettleBill
	HandCount     uint32        // for SettleBill: hand number this session
	BillReason    ResultReason  // for SettleBill: why the hand ended
	KickReasonID  uint32        // for KickPlayer
}
