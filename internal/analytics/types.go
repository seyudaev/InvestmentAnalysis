package analytics

import "time"

type AssetType string

const (
	AssetShare    AssetType = "share"
	AssetBond     AssetType = "bond"
	AssetETF      AssetType = "etf"
	AssetCurrency AssetType = "currency"
	AssetOther    AssetType = "other"
)

type Position struct {
	FIGI         string
	Ticker       string
	Name         string
	AssetType    AssetType
	Quantity     float64
	CurrentPrice float64
	TotalValue   float64
	Currency     string
	YieldPct     float64
}

type PortfolioSnapshot struct {
	AccountID      string
	TotalValue     float64
	Currency       string
	Positions      []Position
	ExpectedYield  float64
	UpdatedAt      time.Time
}

type AllocationItem struct {
	AssetType  AssetType
	Value      float64
	Percent    float64
	TargetPct  float64
	Deviation  float64 // actual - target
}

type SignalAction string

const (
	SignalBuy        SignalAction = "buy"
	SignalSell       SignalAction = "sell"
	SignalHold       SignalAction = "hold"
	SignalOverweight SignalAction = "overweight"
	SignalUnderweight SignalAction = "underweight"
	SignalRisk       SignalAction = "risk"
)

type Signal struct {
	Ticker  string
	Name    string
	Action  SignalAction
	Reason  string
}

type PortfolioAnalysis struct {
	Snapshot    PortfolioSnapshot
	Allocations []AllocationItem
	Signals     []Signal
	TopHoldings []Position
}
