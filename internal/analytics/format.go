package analytics

import (
	"fmt"
	"strings"
)

func FormatPortfolioReport(a PortfolioAnalysis) string {
	var b strings.Builder

	s := a.Snapshot
	fmt.Fprintf(&b, "📊 *Портфель*\n")
	fmt.Fprintf(&b, "Стоимость: *%.2f %s*\n", s.TotalValue, s.Currency)
	if s.ExpectedYield != 0 {
		fmt.Fprintf(&b, "Ожидаемая доходность: %.2f%%\n", s.ExpectedYield)
	}
	fmt.Fprintf(&b, "Позиций: %d\n\n", len(s.Positions))

	if len(a.Allocations) > 0 {
		b.WriteString("*Распределение по классам:*\n")
		for _, al := range a.Allocations {
			label := assetTypeLabel(al.AssetType)
			if al.TargetPct > 0 {
				fmt.Fprintf(&b, "• %s: %.1f%% (цель %.1f%%)\n", label, al.Percent, al.TargetPct)
			} else {
				fmt.Fprintf(&b, "• %s: %.1f%%\n", label, al.Percent)
			}
		}
		b.WriteString("\n")
	}

	if len(a.TopHoldings) > 0 {
		b.WriteString("*Топ позиции:*\n")
		for i, p := range a.TopHoldings {
			fmt.Fprintf(&b, "%d. %s — %.2f %s (%.0f шт.)\n",
				i+1, p.Ticker, p.TotalValue, p.Currency, p.Quantity)
		}
	}

	return b.String()
}

func FormatSignalsReport(signals []Signal) string {
	if len(signals) == 0 {
		return "✅ Сигналов нет — портфель в пределах целевой аллокации."
	}

	var b strings.Builder
	b.WriteString("📡 *Сигналы*\n\n")

	actionEmoji := map[SignalAction]string{
		SignalBuy:         "🟢",
		SignalSell:        "🔴",
		SignalHold:        "⚪",
		SignalOverweight:  "🟡",
		SignalUnderweight: "🔵",
		SignalRisk:        "⚠️",
	}

	for _, s := range signals {
		emoji := actionEmoji[s.Action]
		if emoji == "" {
			emoji = "•"
		}
		fmt.Fprintf(&b, "%s *%s* (%s)\n%s\n\n", emoji, s.Name, s.Action, s.Reason)
	}

	return b.String()
}

func FormatRebalanceData(a PortfolioAnalysis) string {
	var b strings.Builder

	b.WriteString("Данные для ребалансировки:\n\n")
	fmt.Fprintf(&b, "Общая стоимость: %.2f %s\n\n", a.Snapshot.TotalValue, a.Snapshot.Currency)

	b.WriteString("Текущая аллокация vs цель:\n")
	for _, al := range a.Allocations {
		label := assetTypeLabel(al.AssetType)
		if al.TargetPct > 0 {
			fmt.Fprintf(&b, "- %s: %.1f%% (цель %.1f%%, отклонение %+.1f%%)\n",
				label, al.Percent, al.TargetPct, al.Deviation)
		}
	}

	if len(a.Signals) > 0 {
		b.WriteString("\nСигналы:\n")
		for _, s := range a.Signals {
			fmt.Fprintf(&b, "- [%s] %s: %s\n", s.Action, s.Name, s.Reason)
		}
	}

	return b.String()
}
