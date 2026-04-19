package table

import "github.com/qbgames/qbdz-game/internal/player"

// MsgKind identifies the type of an inbound table message.
type MsgKind int

const (
	MsgKindPlayerEnter    MsgKind = iota
	MsgKindPlayerExit
	MsgKindPlayerOffline
	MsgKindPlayerReconnect
	MsgKindPlayerReady
	MsgKindPlayerSit
	MsgKindPlayerStand
	MsgKindPlayerDiscard  // 放弃游戏
	MsgKindGameMessage    // 来自客户端的游戏内消息
	MsgKindForceEnd       // 上层强制结束
	MsgKindReloadConfig
	MsgKindSetBlindLevel  // SNG 盲注递增
	MsgKindBuyFinish      // rebuy/addon 完成
	// Synchronous query kinds — caller blocks on Reply channel for response.
	MsgKindGetRankedPlayers // → []uint32 user IDs ordered by chips
	MsgKindGetCapacity      // → int32 max seat count
)

// QueryReply is the channel type used by synchronous query messages.
// The table goroutine sends exactly one value then the caller reads it.
type QueryReply chan any

// Msg is the value type sent through the table inbox channel.
// All fields are value types (no pointer aliasing) to ensure safe
// cross-goroutine delivery without copying.
type Msg struct {
	Kind MsgKind

	// For PlayerEnter / PlayerSit
	UserID    uint32
	Chair     uint8
	NickName  string
	Avatar    string
	RoleID    uint32
	ChipDesk  uint64 // 带入桌上的筹码

	// For GameMessage
	Cmd     uint32
	Payload []byte

	// For SetBlindLevel
	BlindLevel uint32

	// For PlayerEnter: initial player status
	Stat player.StatType

	// For synchronous queries: caller provides a buffered-1 channel and waits
	// for the table goroutine to send the result.
	Reply QueryReply
}

// Common Protobuf command IDs for Texas Hold'em.
// These match C++ dwCMD values used in DZGame.pb.h / OnGameMsg dispatch.
// Fill in actual values from DZGame.proto when it is available.
const (
	// Client → Server
	CmdReqBet        uint32 = 1001
	CmdReqCall       uint32 = 1002
	CmdReqRaise      uint32 = 1003
	CmdReqCheck      uint32 = 1004
	CmdReqFold       uint32 = 1005
	CmdReqAllIn      uint32 = 1006
	CmdReqBuyChip    uint32 = 1007
	CmdReqBaseInfo   uint32 = 1008
	CmdReqShowCard   uint32 = 1009
	CmdReqAutoAction uint32 = 1010
	CmdReqRebuy      uint32 = 1011
	CmdReqAddon      uint32 = 1012

	// Server → Client
	CmdNotifyGameStart  uint32 = 2001
	CmdNotifyDealCards  uint32 = 2002
	CmdNotifyAction     uint32 = 2003
	CmdNotifyPot        uint32 = 2004
	CmdNotifyShowdown   uint32 = 2005
	CmdNotifyGameResult uint32 = 2006
	CmdNotifyBlindInfo  uint32 = 2007
	CmdNotifyPlayerInfo uint32 = 2008
	CmdNotifyTableInfo  uint32 = 2009
)
