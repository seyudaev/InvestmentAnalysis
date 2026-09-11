package analytics

import (
	"fmt"
	"sort"
)

const deviationThreshold = 5.0 // percent

func AnalyzePortfolio(snapshot PortfolioSnapshot, targets map[AssetType]float64) PortfolioAnalysis {
	allocations := computeAllocations(snapshot, targets)
	signals := generateSignals(allocations, snapshot.Positions)
	topHoldings := topPositions(snapshot.Positions, 5)

	return PortfolioAnalysis{
		Snapshot:    snapshot,
		Allocations: allocations,
		Signals:     signals,
		TopHoldings: topHoldings,
	}
}

func computeAllocations(snapshot PortfolioSnapshot, targets map[AssetType]float64) []AllocationItem {
	typeTotals := make(map[AssetType]float64)
	for _, p := range snapshot.Positions {
		typeTotals[p.AssetType] += p.TotalValue
	}

	total := snapshot.TotalValue
	if total == 0 {
		total = 1
	}

	seen := make(map[AssetType]bool)
	var items []AllocationItem

	for at, value := range typeTotals {
		pct := value / total * 100
		target := targets[at]
		items = append(items, AllocationItem{
			AssetType: at,
			Value:     value,
			Percent:   pct,
			TargetPct: target,
			Deviation: pct - target,
		})
		seen[at] = true
	}

	for at, target := range targets {
		if !seen[at] && target > 0 {
			items = append(items, AllocationItem{
				AssetType: at,
				TargetPct: target,
				Deviation: -target,
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Percent > items[j].Percent
	})

	return items
}

func generateSignals(allocations []AllocationItem, positions []Position) []Signal {
	var signals []Signal

	for _, a := range allocations {
		if a.TargetPct == 0 {
			continue
		}
		if a.Deviation > deviationThreshold {
			signals = append(signals, Signal{
				Ticker: string(a.AssetType),
				Name:   assetTypeLabel(a.AssetType),
				Action: SignalOverweight,
				Reason: fmt.Sprintf("Перевес: %.1f%% (цель %.1f%%, отклонение +%.1f%%)", a.Percent, a.TargetPct, a.Deviation),
			})
		} else if a.Deviation < -deviationThreshold {
			signals = append(signals, Signal{
				Ticker: string(a.AssetType),
				Name:   assetTypeLabel(a.AssetType),
				Action: SignalUnderweight,
				Reason: fmt.Sprintf("Недовес: %.1f%% (цель %.1f%%, отклонение %.1f%%)", a.Percent, a.TargetPct, a.Deviation),
			})
		}
	}

	// Concentration risk: single position > 20% of portfolio
	total := 0.0
	for _, p := range positions {
		total += p.TotalValue
	}
	if total > 0 {
		for _, p := range positions {
			share := p.TotalValue / total * 100
			if share > 20 {
				signals = append(signals, Signal{
					Ticker: p.Ticker,
					Name:   p.Name,
					Action: SignalRisk,
					Reason: fmt.Sprintf("Концентрация риска: %.1f%% портфеля", share),
				})
			}
		}
	}

	return signals
}

func topPositions(positions []Position, n int) []Position {
	sorted := make([]Position, len(positions))
	copy(sorted, positions)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].TotalValue > sorted[j].TotalValue
	})
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}

func assetTypeLabel(at AssetType) string {
	switch at {
	case AssetShare:
		return "Акции"
	case AssetBond:
		return "Облигации"
	case AssetETF:
		return "ETF/Фонды"
	case AssetCurrency:
		return "Валюта"
	default:
		return "Прочее"
	}
}

func DefaultTargets() map[AssetType]float64 {
	return map[AssetType]float64{
		AssetShare:    50,
		AssetBond:     30,
		AssetETF:      10,
		AssetCurrency: 10,
	}
}
