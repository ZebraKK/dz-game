package protocol

// AuthChallengePayload is sent before authentication.
type AuthChallengePayload struct {
	ServerID  string `json:"serverId"`
	Challenge string `json:"challenge"`
	ExpiresAt string `json:"expiresAt"`
}

type AuthPayload struct {
	Method    string `json:"method"`
	PlayerID  string `json:"playerId"`
	KeyID     string `json:"keyId"`
	Signature string `json:"signature"`
}

type AuthOKPayload struct {
	PlayerID    string `json:"playerId"`
	DisplayName string `json:"displayName"`
	SessionID   string `json:"sessionId"`
}

type JoinPayload struct {
	TableID string `json:"tableId"`
}

type JoinOKPayload struct {
	TableID    string `json:"tableId"`
	Role       string `json:"role"`
	Seat       *int   `json:"seat"`
	PendingSit bool   `json:"pendingSit"`
}

type RuntimeConfig struct {
	MaxSeats          int    `json:"maxSeats"`
	SmallBlind        uint64 `json:"smallBlind"`
	BigBlind          uint64 `json:"bigBlind"`
	Ante              uint64 `json:"ante"`
	StartingStack     uint64 `json:"startingStack"`
	RakePercent       int    `json:"rakePercent"`
	ActionTimeoutMs   int64  `json:"actionTimeoutMs"`
	DisconnectGraceMs int64  `json:"disconnectGraceMs"`
	StartCountdownMs  int64  `json:"startCountdownMs"`
	ResultDisplayMs   int64  `json:"resultDisplayMs"`
	ReadyTimeoutMs    int64  `json:"readyTimeoutMs"`
}

type PotView struct {
	Total uint64 `json:"total"`
}

type TurnView struct {
	TurnID   string `json:"turnId"`
	Seat     int    `json:"seat"`
	PlayerID string `json:"playerId"`
	Deadline string `json:"deadline"`
}

type LastActionView struct {
	Type   string `json:"type"`
	Amount uint64 `json:"amount,omitempty"`
	Auto   bool   `json:"auto"`
}

type SeatView struct {
	Seat          int             `json:"seat"`
	Occupancy     string          `json:"occupancy"`
	PlayerID      *string         `json:"playerId"`
	DisplayName   *string         `json:"displayName"`
	Participation *string         `json:"participation"`
	Connection    *string         `json:"connection"`
	Ready         bool            `json:"ready"`
	Stack         uint64          `json:"stack"`
	StreetBet     uint64          `json:"streetBet"`
	HandCommitted uint64          `json:"handCommitted"`
	IsDealer      bool            `json:"isDealer"`
	IsSmallBlind  bool            `json:"isSmallBlind"`
	IsBigBlind    bool            `json:"isBigBlind"`
	LastAction    *LastActionView `json:"lastAction"`
	RevealedCards []string        `json:"revealedCards"`
}

type LegalAction struct {
	Type      string `json:"type"`
	Amount    uint64 `json:"amount,omitempty"`
	MinAmount uint64 `json:"minAmount,omitempty"`
	MaxAmount uint64 `json:"maxAmount,omitempty"`
}

type SelfView struct {
	PlayerID     string        `json:"playerId"`
	DisplayName  string        `json:"displayName"`
	Role         string        `json:"role"`
	Seat         *int          `json:"seat"`
	HoleCards    []string      `json:"holeCards"`
	PendingSit   bool          `json:"pendingSit"`
	Ready        bool          `json:"ready"`
	LegalActions []LegalAction `json:"legalActions"`
	TurnID       *string       `json:"turnId"`
}

type SnapshotPayload struct {
	TableID        string        `json:"tableId"`
	HandID         *string       `json:"handId"`
	Phase          string        `json:"phase"`
	Config         RuntimeConfig `json:"config"`
	ButtonSeat     *int          `json:"buttonSeat"`
	SmallBlindSeat *int          `json:"smallBlindSeat"`
	BigBlindSeat   *int          `json:"bigBlindSeat"`
	Board          []string      `json:"board"`
	Pot            PotView       `json:"pot"`
	CurrentTurn    *TurnView     `json:"currentTurn"`
	ReadyDeadline  *string       `json:"readyDeadline"`
	Seats          []SeatView    `json:"seats"`
	You            SelfView      `json:"you"`
}

type SitPayload struct {
	PreferredSeat *int `json:"preferredSeat,omitempty"`
}

type SitPendingPayload struct {
	PreferredSeat *int   `json:"preferredSeat"`
	Effective     string `json:"effective"`
	Message       string `json:"message"`
}

type SitCancelledPayload struct {
	Message string `json:"message"`
}

type SeatAssignedPayload struct {
	Seat        int    `json:"seat"`
	PlayerID    string `json:"playerId"`
	DisplayName string `json:"displayName"`
	Effective   string `json:"effective"`
}

type PlayerSitOutPayload struct {
	Seat     int    `json:"seat"`
	PlayerID string `json:"playerId"`
	SitOut   bool   `json:"sitout"`
}

type PlayerLeftPayload struct {
	Seat     int    `json:"seat"`
	PlayerID string `json:"playerId"`
}

type ReadyCheckPayload struct {
	Deadline string `json:"deadline"`
}

type PlayerReadyPayload struct {
	Seat     int    `json:"seat"`
	PlayerID string `json:"playerId"`
	Ready    bool   `json:"ready"`
}

type PlayerStoodPayload struct {
	Seat     int    `json:"seat"`
	PlayerID string `json:"playerId"`
	Reason   string `json:"reason"`
}

type HandStartedPayload struct {
	HandID             string            `json:"handId"`
	ButtonSeat         int               `json:"buttonSeat"`
	SmallBlindSeat     int               `json:"smallBlindSeat"`
	BigBlindSeat       int               `json:"bigBlindSeat"`
	ParticipatingSeats []int             `json:"participatingSeats"`
	StartingStacks     map[string]uint64 `json:"startingStacks"`
}

type BlindPostedPayload struct {
	HandID     string `json:"handId"`
	Seat       int    `json:"seat"`
	PlayerID   string `json:"playerId"`
	Blind      string `json:"blind"`
	Amount     uint64 `json:"amount"`
	StackAfter uint64 `json:"stackAfter"`
	Pot        uint64 `json:"pot"`
}

type HoleCardsPayload struct {
	HandID string   `json:"handId"`
	Cards  []string `json:"cards"`
}

type ActionRequestPayload struct {
	HandID       string        `json:"handId"`
	TurnID       string        `json:"turnId"`
	PlayerID     string        `json:"playerId"`
	Seat         int           `json:"seat"`
	Deadline     string        `json:"deadline"`
	ToCall       uint64        `json:"toCall"`
	Pot          uint64        `json:"pot"`
	LegalActions []LegalAction `json:"legalActions"`
}

type PlayerAction struct {
	Type   string `json:"type"`
	Amount uint64 `json:"amount,omitempty"`
}

type ActionPayload struct {
	ActionID string       `json:"actionId"`
	HandID   string       `json:"handId"`
	TurnID   string       `json:"turnId"`
	Action   PlayerAction `json:"action"`
}

type ActionResultPayload struct {
	HandID         string       `json:"handId"`
	ActionID       string       `json:"actionId"`
	PlayerID       string       `json:"playerId"`
	Seat           int          `json:"seat"`
	Action         PlayerAction `json:"action"`
	Auto           bool         `json:"auto"`
	Pot            uint64       `json:"pot"`
	StackAfter     uint64       `json:"stackAfter"`
	StreetBetAfter uint64       `json:"streetBetAfter"`
}

type BoardDealtPayload struct {
	HandID string   `json:"handId"`
	Street string   `json:"street"`
	Cards  []string `json:"cards"`
	Board  []string `json:"board"`
	Pot    uint64   `json:"pot"`
}

type ShowdownReveal struct {
	Seat     int      `json:"seat"`
	PlayerID string   `json:"playerId"`
	Cards    []string `json:"cards"`
	HandRank string   `json:"handRank"`
}

type ShowdownRevealedPayload struct {
	HandID  string           `json:"handId"`
	Board   []string         `json:"board"`
	Reveals []ShowdownReveal `json:"reveals"`
}

type PotResult struct {
	Index         int    `json:"index"`
	Amount        uint64 `json:"amount"`
	EligibleSeats []int  `json:"eligibleSeats"`
	WinnerSeats   []int  `json:"winnerSeats"`
}

type WinnerResult struct {
	Seat     int    `json:"seat"`
	PlayerID string `json:"playerId"`
	Amount   uint64 `json:"amount"`
	HandRank string `json:"handRank"`
}

type HandSettledPayload struct {
	HandID      string            `json:"handId"`
	Board       []string          `json:"board"`
	Pots        []PotResult       `json:"pots"`
	Winners     []WinnerResult    `json:"winners"`
	FinalStacks map[string]uint64 `json:"finalStacks"`
	Deltas      map[string]int64  `json:"deltas"`
}

type PingPayload struct {
	Nonce string `json:"nonce"`
}

type PongPayload struct {
	Nonce string `json:"nonce"`
}

type StatusPayload struct{}

type EmptyPayload struct{}

type ErrorPayload struct {
	Code            string         `json:"code"`
	Message         string         `json:"message"`
	Retryable       bool           `json:"retryable"`
	RequestID       string         `json:"requestId,omitempty"`
	RefActionID     string         `json:"refActionId,omitempty"`
	RefStateVersion int64          `json:"refStateVersion,omitempty"`
	Details         map[string]any `json:"details,omitempty"`
	RetryAfterMs    int64          `json:"retryAfterMs,omitempty"`
}

const (
	ErrAuthTimeout         = "AUTH_TIMEOUT"
	ErrAuthFailed          = "AUTH_FAILED"
	ErrUnsupportedProtocol = "UNSUPPORTED_PROTOCOL"
	ErrNotAuthenticated    = "NOT_AUTHENTICATED"
	ErrSessionReplaced     = "SESSION_REPLACED"
	ErrRateLimited         = "RATE_LIMITED"
	ErrMalformedMessage    = "MALFORMED_MESSAGE"
	ErrInternal            = "INTERNAL_ERROR"
	ErrNotJoined           = "NOT_JOINED"
	ErrAlreadySeated       = "ALREADY_SEATED"
	ErrNotSeated           = "NOT_SEATED"
	ErrTableFull           = "TABLE_FULL"
	ErrInvalidSeat         = "INVALID_SEAT"
	ErrSeatOccupied        = "SEAT_OCCUPIED"
	ErrSitPending          = "SIT_PENDING"
	ErrNoChips             = "NO_CHIPS"
	ErrReadyCheckNotOpen   = "READY_CHECK_NOT_OPEN"
	ErrHandInProgress      = "HAND_IN_PROGRESS"
	ErrNotYourTurn         = "NOT_YOUR_TURN"
	ErrStaleTurn           = "STALE_TURN"
	ErrInvalidAction       = "INVALID_ACTION"
	ErrActionTimeout       = "ACTION_TIMEOUT"
	ErrIDConflict          = "IDEMPOTENCY_CONFLICT"
	ErrHeartbeatTimeout    = "HEARTBEAT_TIMEOUT"
)
