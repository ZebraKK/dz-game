// Package config defines all configuration structs for the QBDZ game server
// and provides a YAML-based loader with hot-reload support.
//
// This package replaces the C++ TDR/XML configuration framework.
// All struct fields map 1:1 to the C++ _tagSceneConfig and related structs
// in Source/SoGameLogic/SoConfig.h to ensure behavioral parity.
package config

// GameType identifies the game mode.
// Values match C++ eumGameType in SoGameCommon.h.
type GameType int

const (
	GameClassic     GameType = 1  // GT_CLASSIC  经典场
	GameTycoon      GameType = 2  // GT_TYCOON   大亨场
	GameSNG         GameType = 3  // GT_SNG      Sit-and-Go
	GameMTT         GameType = 4  // GT_MTT      Multi-Table Tournament
	GameTrain       GameType = 5  // GT_TRAIN    训练营
	GameFriend      GameType = 6  // GT_FRIEND   朋友桌
	GameFriendScore GameType = 7  // GT_FRIEND_SCORE 积分场
)

// SceneType constants map scene IDs to game variants.
// Matches C++ ST_* defines in SoGameCommon.h.
const (
	SceneClassic  = 10301 // ST_CLASSIC
	SceneTycoon   = 10302 // ST_TYCOON
	SceneSNG      = 10303 // ST_SNG
	SceneMTT      = 10304 // ST_MTT
	SceneTrain    = 10305 // ST_TRAIN
	SceneFriend   = 10306 // ST_FRIEND
	SceneCowboy   = 10390 // ST_COWBOY
)

// SceneConfig holds all per-room game parameters.
// Maps to C++ _tagSceneConfig / SCENE_CONFIG in SoConfig.h.
type SceneConfig struct {
	// Logging (not used in Go version, kept for parity)
	LogFlag    uint32 `yaml:"logFlag"`
	ErrLogFlag uint32 `yaml:"errLogFlag"`
	LogPath    string `yaml:"logPath"`

	// Room identity
	AreaID     uint32   `yaml:"areaID"`
	SceneLevel uint32   `yaml:"sceneLevel"`
	GameType   GameType `yaml:"gameType"`
	SceneName  string   `yaml:"sceneName"`
	MatchType  uint32   `yaml:"matchType"`

	// Table setup
	SeatCount     uint32 `yaml:"seatCount"`     // 座位数量，默认9
	ServiceCharge uint32 `yaml:"serviceCharge"` // 服务费（抽水）比例
	WaitForStart  uint32 `yaml:"waitForStart"`  // 等待开始的秒数
	ChipRatio     uint32 `yaml:"chipRatio"`     // 筹码换算比例 money/chip

	// Timing
	ActionTimeSec uint32 `yaml:"actionTimeSec"` // 玩家行动超时秒数

	// Bet limits
	BetLimit uint64 `yaml:"betLimit"` // 单次下注上限

	// Buy-in / auto
	AutoSitDown bool   `yaml:"autoSitDown"` // 是否自动入座
	AutoBuyIn   bool   `yaml:"autoBuyIn"`   // 是否自动买入
	MinBuyIn    uint64 `yaml:"minBuyIn"`    // 最小带入筹码
	MaxBuyIn    uint64 `yaml:"maxBuyIn"`    // 最大带入筹码
	MinCoin     uint64 `yaml:"minCoin"`     // 最小金币要求
	MaxCoin     uint64 `yaml:"maxCoin"`     // 最大金币要求
	DefaultBuyIn uint32 `yaml:"defaultBuyIn"` // 默认带入

	// Blinds
	SmallBlind uint64 `yaml:"smallBlind"`
	BigBlind   uint64 `yaml:"bigBlind"`
	Ante       uint32 `yaml:"ante"`
	PotLimit   uint32 `yaml:"potLimit"`   // 0=无限注，非0=底池限注
	ForceBetChips uint64 `yaml:"forceBetChips"` // 强制下注筹码
	CanForceBB uint32 `yaml:"canForceBB"` // 是否强交大盲

	// Rebuy (重购)
	CanRebuy          bool   `yaml:"canRebuy"`
	RebuyFee          uint32 `yaml:"rebuyFee"`
	RebuyChip         uint32 `yaml:"rebuyChip"`
	RebuyCountMax     uint32 `yaml:"rebuyCountMax"`
	RebuyBlindLevelMin uint32 `yaml:"rebuyBlindLevelMin"`
	RebuyBlindLevelMax uint32 `yaml:"rebuyBlindLevelMax"`
	RebuyWaitTime     uint32 `yaml:"rebuyWaitTime"`
	RebuyChipLimit    uint32 `yaml:"rebuyChipLimit"`

	// Addon (增购)
	CanAddOn          bool   `yaml:"canAddOn"`
	AddonFee          uint32 `yaml:"addonFee"`
	AddonChip         uint32 `yaml:"addonChip"`
	AddonCountMax     uint32 `yaml:"addonCountMax"`
	AddonBlindLevelMin uint32 `yaml:"addonBlindLevelMin"`
	AddonBlindLevelMax uint32 `yaml:"addonBlindLevelMax"`
	AddonWaitTime     uint32 `yaml:"addonWaitTime"`

	// Misc
	ExcludeCards uint32 `yaml:"excludeCards"`
}

// DefaultSceneConfig returns a SceneConfig with sensible defaults.
func DefaultSceneConfig() SceneConfig {
	return SceneConfig{
		LogFlag:       1,
		ErrLogFlag:    1,
		SeatCount:     9,
		ActionTimeSec: 30,
		SmallBlind:    50,
		BigBlind:      100,
		MinBuyIn:      1000,
		MaxBuyIn:      20000,
		ChipRatio:     1,
	}
}

// SNGReward describes a prize for a SNG finish position.
type SNGReward struct {
	Rank          uint32 `yaml:"rank"`
	PropertyID    uint32 `yaml:"propertyID"`
	PropertyCount uint32 `yaml:"propertyCount"`
}

// SNGInfoConfig holds Sit-and-Go tournament parameters.
// Maps to C++ _tagSNGInfoConfig / SNG_INFO_CONFIG in SoConfig.h.
type SNGInfoConfig struct {
	SceneLevel    uint32      `yaml:"sceneLevel"`
	InitChips     uint32      `yaml:"initChips"`
	TicketPrice   uint32      `yaml:"ticketPrice"`
	ServiceCharge uint32      `yaml:"serviceCharge"`
	RewardContent string      `yaml:"rewardContent"`
	Rewards       []SNGReward `yaml:"rewards"`
}

// BlindInfoConfig holds one level of a blind schedule (SNG escalation).
// Maps to C++ _tagBlindInfoConfig / BLIND_INFO_CONFIG in SoConfig.h.
type BlindInfoConfig struct {
	Valid       bool   `yaml:"valid"`
	SceneLevel  uint32 `yaml:"sceneLevel"`
	Level       uint32 `yaml:"level"`       // 盲注级别（从1开始）
	SmallBlind  uint32 `yaml:"smallBlind"`
	BigBlind    uint32 `yaml:"bigBlind"`
	Ante        uint32 `yaml:"ante"`
	DurationSec uint32 `yaml:"durationSec"` // 该级别持续秒数
}

// SystemConfig holds server-level feature flags.
// Maps to C++ _tagSystemConfig / SYSTEM_CONFIG in SoConfig.h.
type SystemConfig struct {
	EnableRookieTraining     bool   `yaml:"enableRookieTraining"`
	RookieTrainingRoundLimit uint32 `yaml:"rookieTrainingRoundLimit"`
}

// FriendTableConfig holds private room (好友桌) parameters.
// Maps to C++ FRIEND_TABLE_CONFIG in SoConfig.h.
type FriendTableConfig struct {
	SceneLevel  uint32   `yaml:"sceneLevel"`
	RoomType    uint32   `yaml:"roomType"`
	SmallBlind  uint32   `yaml:"smallBlind"`
	BigBlind    uint32   `yaml:"bigBlind"`
	AnteChip    uint32   `yaml:"anteChip"`
	BringChips  uint32   `yaml:"bringChips"`
	BureauFund  uint32   `yaml:"bureauFund"`  // 组局基金
	MaxExchange uint32   `yaml:"maxExchange"` // 带入上限
	BureauRate  uint32   `yaml:"bureauRate"`  // 积分场扣除比例
	BlindArr    []uint32 `yaml:"blindArr"`
	AnteArr     []uint32 `yaml:"anteArr"`
	ExchangeArr []uint32 `yaml:"exchangeArr"`
}

// RobotHandCard describes a hand range entry for robot AI.
type RobotHandCard struct {
	Card0   byte   `yaml:"card0"`
	Card1   byte   `yaml:"card1"`
	WinRate uint32 `yaml:"winRate"` // 胜率（百分比整数）
}

// RobotOutsRate maps an outs count to turn/river completion probability.
// Maps to C++ RobotAI_OUTSRATE_INFO in SoConfig.h.
type RobotOutsRate struct {
	OutsCount uint32 `yaml:"outsCount"`
	TurnRate  uint32 `yaml:"turnRate"`  // 百分比
	RiverRate uint32 `yaml:"riverRate"` // 百分比
}

// CowboyBetCoinInfo describes one betting denomination tier for Cowboy.
type CowboyBetCoinInfo struct {
	MinCoin uint64 `yaml:"minCoin"`
	BetCoin uint64 `yaml:"betCoin"`
}

// CowboySceneConfig holds Cowboy game variant parameters.
// Maps to C++ COWBOY_SCENE_CONFIG in SoConfig.h.
type CowboySceneConfig struct {
	SceneLevel       uint32              `yaml:"sceneLevel"`
	MinCoin          uint64              `yaml:"minCoin"`
	MaxCoin          uint64              `yaml:"maxCoin"`
	SeatCount        uint32              `yaml:"seatCount"`
	RoomCapacity     uint32              `yaml:"roomCapacity"`
	WaitTimeSec      uint32              `yaml:"waitTimeSec"`
	BetTimeSec       uint32              `yaml:"betTimeSec"`
	ResultToStartSec uint32              `yaml:"resultToStartSec"`
	SeatPlayerRefresh uint32             `yaml:"seatPlayerRefresh"`
	TableBetLimit    uint64              `yaml:"tableBetLimit"`
	PlayerBetLimit   uint64              `yaml:"playerBetLimit"`
	BetCoinInfos     []CowboyBetCoinInfo `yaml:"betCoinInfos"`
}

// CowboyOddInfo describes a payout multiplier for one Cowboy bet position.
type CowboyOddInfo struct {
	Position uint32 `yaml:"position"`
	CDCount  uint32 `yaml:"cdCount"`
	Odds     uint32 `yaml:"odds"`
}

// CowboyOddsConfig holds the full payout table for Cowboy.
// Maps to C++ COWBOY_ODDS_CONFIG in SoConfig.h.
type CowboyOddsConfig struct {
	OddInfos []CowboyOddInfo `yaml:"oddInfos"`
}

// GetOddsByPosition returns the odds multiplier for a bet position.
func (c *CowboyOddsConfig) GetOddsByPosition(position uint32) uint32 {
	for _, o := range c.OddInfos {
		if o.Position == position {
			return o.Odds
		}
	}
	return 0
}

// JackpotConfig holds jackpot trigger hand types.
type JackpotConfig struct {
	CardTypeList []uint32 `yaml:"cardTypeList"`
}

// AllConfig is the complete configuration loaded from YAML.
type AllConfig struct {
	Scene       SceneConfig       `yaml:"scene"`
	SNG         SNGInfoConfig     `yaml:"sng"`
	Blinds      []BlindInfoConfig `yaml:"blinds"` // 按 level 索引
	System      SystemConfig      `yaml:"system"`
	FriendTable FriendTableConfig `yaml:"friendTable"`
	RobotCards  []RobotHandCard   `yaml:"robotCards"`
	RobotOuts   []RobotOutsRate   `yaml:"robotOuts"`
	Cowboy      CowboySceneConfig `yaml:"cowboy"`
	CowboyOdds  CowboyOddsConfig  `yaml:"cowboyOdds"`
	Jackpot     JackpotConfig     `yaml:"jackpot"`
}

// BlindByLevel returns the blind config for the given level (1-based).
// Returns nil if the level is not found or invalid.
func (a *AllConfig) BlindByLevel(level uint32) *BlindInfoConfig {
	for i := range a.Blinds {
		if a.Blinds[i].Level == level && a.Blinds[i].Valid {
			return &a.Blinds[i]
		}
	}
	return nil
}

// RobotOutsByOuts returns the outs-rate entry for a given outs count.
func (a *AllConfig) RobotOutsByOuts(outsCount uint32) *RobotOutsRate {
	for i := range a.RobotOuts {
		if a.RobotOuts[i].OutsCount == outsCount {
			return &a.RobotOuts[i]
		}
	}
	return nil
}
