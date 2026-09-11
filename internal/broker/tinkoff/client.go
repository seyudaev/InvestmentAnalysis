package tinkoff

import (
	"context"
	"fmt"

	"github.com/seyud/investment-analysis/internal/analytics"
	investgo "github.com/tinkoff/invest-api-go-sdk/investgo"
	pb "github.com/tinkoff/invest-api-go-sdk/proto"
)

type Client struct {
	endpoint string
}

func NewClient(endpoint string) *Client {
	return &Client{endpoint: endpoint}
}

func (c *Client) GetPortfolio(ctx context.Context, token, accountID string) (*analytics.PortfolioSnapshot, error) {
	client, err := c.newSDKClient(ctx, token)
	if err != nil {
		return nil, err
	}
	defer client.Stop()

	if accountID == "" {
		accountID, err = c.firstAccountID(client)
		if err != nil {
			return nil, err
		}
	}

	opsClient := client.NewOperationsServiceClient()
	portfolioResp, err := opsClient.GetPortfolio(accountID, pb.PortfolioRequest_RUB)
	if err != nil {
		return nil, fmt.Errorf("get portfolio: %w", err)
	}

	positionsResp, err := opsClient.GetPositions(accountID)
	if err != nil {
		return nil, fmt.Errorf("get positions: %w", err)
	}

	instrumentsClient := client.NewInstrumentsServiceClient()

	snapshot := &analytics.PortfolioSnapshot{
		AccountID: accountID,
		Currency:  "RUB",
	}

	if total := portfolioResp.GetTotalAmountPortfolio(); total != nil {
		snapshot.TotalValue = total.ToFloat()
	}

	if y := portfolioResp.GetExpectedYield(); y != nil {
		snapshot.ExpectedYield = y.ToFloat()
	}

	for _, p := range portfolioResp.GetPositions() {
		pos := c.parsePortfolioPosition(p, instrumentsClient)
		if pos.Quantity > 0 || pos.TotalValue > 0 {
			snapshot.Positions = append(snapshot.Positions, pos)
		}
	}

	for _, m := range positionsResp.GetMoney() {
		value := m.ToFloat()
		if value > 0 {
			snapshot.Positions = append(snapshot.Positions, analytics.Position{
				Ticker:     m.GetCurrency(),
				Name:       "Валюта " + m.GetCurrency(),
				AssetType:  analytics.AssetCurrency,
				Quantity:   value,
				TotalValue: value,
				Currency:   m.GetCurrency(),
			})
		}
	}

	// Recalculate total if portfolio total is zero
	if snapshot.TotalValue == 0 {
		for _, p := range snapshot.Positions {
			snapshot.TotalValue += p.TotalValue
		}
	}

	return snapshot, nil
}

func (c *Client) GetAccounts(ctx context.Context, token string) ([]AccountInfo, error) {
	client, err := c.newSDKClient(ctx, token)
	if err != nil {
		return nil, err
	}
	defer client.Stop()

	accountsResp, err := client.NewUsersServiceClient().GetAccounts()
	if err != nil {
		return nil, fmt.Errorf("get accounts: %w", err)
	}

	var result []AccountInfo
	for _, a := range accountsResp.GetAccounts() {
		result = append(result, AccountInfo{
			ID:   a.GetId(),
			Name: a.GetName(),
			Type: a.GetType().String(),
		})
	}
	return result, nil
}

type AccountInfo struct {
	ID   string
	Name string
	Type string
}

func (c *Client) newSDKClient(ctx context.Context, token string) (*investgo.Client, error) {
	cfg := investgo.Config{
		EndPoint: c.endpoint,
		Token:    token,
		AppName:  "investment-analysis-bot",
	}

	client, err := investgo.NewClient(ctx, cfg, newSDKLogger())
	if err != nil {
		return nil, fmt.Errorf("create tinkoff client: %w", err)
	}
	return client, nil
}

func (c *Client) firstAccountID(client *investgo.Client) (string, error) {
	accountsResp, err := client.NewUsersServiceClient().GetAccounts()
	if err != nil {
		return "", fmt.Errorf("get accounts: %w", err)
	}
	accounts := accountsResp.GetAccounts()
	if len(accounts) == 0 {
		return "", fmt.Errorf("no accounts found")
	}
	return accounts[0].GetId(), nil
}

func (c *Client) parsePortfolioPosition(p *pb.PortfolioPosition, ic *investgo.InstrumentsServiceClient) analytics.Position {
	figi := p.GetFigi()
	qty := quotationToFloat(p.GetQuantity())
	currentPrice := moneyToFloat(p.GetCurrentPrice())
	totalValue := qty * currentPrice

	if nkd := p.GetCurrentNkd(); nkd != nil {
		totalValue += nkd.ToFloat()
	}

	pos := analytics.Position{
		FIGI:         figi,
		Quantity:     qty,
		CurrentPrice: currentPrice,
		TotalValue:   totalValue,
		Currency:     "RUB",
		YieldPct:     quotationToFloat(p.GetExpectedYield()),
		AssetType:    mapInstrumentType(p.GetInstrumentType()),
	}

	if figi != "" {
		if share, err := ic.ShareByFigi(figi); err == nil && share.GetInstrument() != nil {
			inst := share.GetInstrument()
			pos.Ticker = inst.GetTicker()
			pos.Name = inst.GetName()
			pos.AssetType = analytics.AssetShare
			return pos
		}
		if bond, err := ic.BondByFigi(figi); err == nil && bond.GetInstrument() != nil {
			inst := bond.GetInstrument()
			pos.Ticker = inst.GetTicker()
			pos.Name = inst.GetName()
			pos.AssetType = analytics.AssetBond
			return pos
		}
		if etf, err := ic.EtfByFigi(figi); err == nil && etf.GetInstrument() != nil {
			inst := etf.GetInstrument()
			pos.Ticker = inst.GetTicker()
			pos.Name = inst.GetName()
			pos.AssetType = analytics.AssetETF
			return pos
		}
	}

	if pos.Ticker == "" {
		pos.Ticker = figi
		pos.Name = figi
	}

	return pos
}

func mapInstrumentType(t string) analytics.AssetType {
	switch t {
	case "share":
		return analytics.AssetShare
	case "bond":
		return analytics.AssetBond
	case "etf":
		return analytics.AssetETF
	case "currency":
		return analytics.AssetCurrency
	default:
		return analytics.AssetOther
	}
}

func quotationToFloat(q *pb.Quotation) float64 {
	if q == nil {
		return 0
	}
	return q.ToFloat()
}

func moneyToFloat(m *pb.MoneyValue) float64 {
	if m == nil {
		return 0
	}
	return m.ToFloat()
}
