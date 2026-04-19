// Package cowboy implements the Cowboy poker game variant.
//
// Game flow (C++ reference: CSoCowboyGameLogic):
//   NoStart → CountdownStart → Start → SendCard → BetBegan → BetEnd → Result → (loop)
//
// In each hand:
//   - 5 community cards + 2 "Cowboy"(dealer) cards + 2 "Cow"(player) cards are dealt
//   - Players bet on the outcome during BetBegan phase
//   - After BetEnd, results are calculated and payouts made
package cowboy

// GameStatus represents the Cowboy game phase.
// Values match C++ eumSoCowboyGameStatus in SoGameCommon.h exactly.
type GameStatus int

const (
	StatusNoStart      GameStatus = 0 // eumSoCbGsNoStart
	StatusCountdown    GameStatus = 1 // eumSoCbGsCountDownStart
	StatusStart        GameStatus = 2 // eumSoCbGsStart
	StatusSendCard     GameStatus = 3 // eumSoCbGsSendCard
	StatusBetBegan     GameStatus = 4 // eumSoCbGsBetBegan
	StatusBetEnd       GameStatus = 5 // eumSoCbGsBetEnd
	StatusResult       GameStatus = 6 // eumSoCbGsResult
)

func (s GameStatus) String() string {
	names := []string{
		"NoStart", "Countdown", "Start", "SendCard", "BetBegan", "BetEnd", "Result",
	}
	if int(s) < len(names) {
		return names[s]
	}
	return "Unknown"
}

// BetPosition identifies a wagerable outcome zone.
// Values match C++ eumSoCowboyBetPosition in SoGameCommon.h exactly.
type BetPosition int

const (
	BetNone        BetPosition = 0  // 未定义
	BetDraw        BetPosition = 1  // 平局
	BetPersonWin   BetPosition = 2  // 人（Cow）赢
	BetCowWin      BetPosition = 3  // 牛仔（Cowboy）赢
	BetFlush       BetPosition = 11 // 同花（边注）
	BetStraight    BetPosition = 12 // 连牌/顺子（边注）
	BetPair        BetPosition = 13 // 对子（边注）
	BetStraightFlush BetPosition = 14 // 同花连牌（边注）
	BetAA          BetPosition = 15 // AA（边注）
	BetHighPair    BetPosition = 21 // 高牌/一对（牌型注）
	BetTwoPair     BetPosition = 22 // 两对（牌型注）
	BetThreeFlush  BetPosition = 23 // 三条/顺子/同花（牌型注）
	BetFullHouse   BetPosition = 24 // 葫芦（牌型注）
	BetQuadPlus    BetPosition = 25 // 金刚/同花顺/皇家同花顺（牌型注）
)

// BetPositionEnd is the sentinel value for range checks.
// Matches C++ eumSoCbBpEnd.
const BetPositionEnd = 26

// WinStatus describes the hand comparison outcome.
// Values match C++ eumSoCowboyWinStatus in SoGameCommon.h exactly.
type WinStatus int

const (
	WinNone   WinStatus = 0 // 未定义
	WinDraw   WinStatus = 1 // 平局
	WinCowboy WinStatus = 2 // 牛仔赢（dealer wins）
	WinCow    WinStatus = 3 // 小牛赢（player wins）
)

func (w WinStatus) String() string {
	switch w {
	case WinDraw:
		return "Draw"
	case WinCowboy:
		return "CowboyWin"
	case WinCow:
		return "CowWin"
	}
	return "None"
}

// TimerID identifies a Cowboy game timer.
type TimerID int

const (
	TimerNone            TimerID = 0
	TimerWaitForStart    TimerID = 1 // 等待玩家开始倒计时
	TimerStart           TimerID = 2 // 游戏开始动画
	TimerSendCard        TimerID = 3 // 发牌动画
	TimerBet             TimerID = 4 // 押注时间
	TimerResultToStart   TimerID = 5 // 结算到下一局的间隔
)

// EventKind describes what the Table actor should do after a Game method call.
type EventKind int

const (
	EventSendAll    EventKind = iota
	EventSendTo
	EventSendExcept
	EventSetTimer
	EventCancelTimer
	EventSettleBet  // pay out one player's bet on one position
	EventGameStart
	EventGameEnd
	EventCowboyBet  // record a bet with the upper server
)

// Event is the return value from all Cowboy game logic methods.
type Event struct {
	Kind          EventKind
	TargetUserID  uint32
	ExcludeUserID uint32
	Cmd           uint32
	Payload       []byte
	TimerID       TimerID
	TimerDuration int64 // milliseconds

	// For EventSettleBet
	BetUserID  uint32
	BetPos     BetPosition
	BetAmount  uint64
	PayAmount  int64 // positive=win, negative=loss (= -betAmount for a loss)

	// For EventCowboyBet
	CowboyBetUserID uint32
	CowboyBetAmount uint64
}

// ResultRecord is one entry in the rolling 30-hand history.
type ResultRecord struct {
	IsValid       bool
	WinStatus     WinStatus
	WinnerCardType int // cards.HandType value of the winning hand
}

// Cowboy command IDs (Server → Client / Client → Server).
// Will be replaced by actual proto cmd values once DZGame.proto is integrated.
const (
	CmdReqBaseInfo       uint32 = 3001
	CmdReqBet            uint32 = 3002
	CmdReqStatisticalInfo uint32 = 3003

	CmdNotifyGameStatus  uint32 = 4001
	CmdNotifyPlayerCards uint32 = 4002
	CmdNotifyBets        uint32 = 4003
	CmdNotifyGameResult  uint32 = 4004
	CmdNotifyBaseInfo    uint32 = 4005
)
