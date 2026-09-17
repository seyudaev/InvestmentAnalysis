package tinkoff

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"strings"

	"github.com/seyud/investment-analysis/internal/analytics"
	pb "github.com/tinkoff/invest-api-go-sdk/proto"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/metadata"
)

const (
	defaultEndpoint = "invest-public-api.tinkoff.ru:443"
	appName         = "investment-analysis-bot"
)

type Client struct {
	endpoint   string
	tlsInsecure bool
}

func NewClient(endpoint string, tlsInsecure bool) *Client {
	return &Client{
		endpoint:    normalizeEndpoint(endpoint),
		tlsInsecure: tlsInsecure,
	}
}

type grpcSession struct {
	conn *grpc.ClientConn
	ctx  context.Context
}

func (s *grpcSession) Close() {
	if s.conn != nil {
		_ = s.conn.Close()
	}
}

func (c *Client) GetPortfolio(ctx context.Context, token, accountID string) (*analytics.PortfolioSnapshot, error) {
	session, err := c.dial(ctx, token)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	users := pb.NewUsersServiceClient(session.conn)
	ops := pb.NewOperationsServiceClient(session.conn)
	instruments := pb.NewInstrumentsServiceClient(session.conn)

	if accountID == "" {
		accountID, err = firstAccountID(session.ctx, users)
		if err != nil {
			return nil, err
		}
	}

	portfolioResp, err := ops.GetPortfolio(session.ctx, &pb.PortfolioRequest{
		AccountId: accountID,
		Currency:  pb.PortfolioRequest_RUB,
	})
	if err != nil {
		return nil, fmt.Errorf("get portfolio: %w", err)
	}

	positionsResp, err := ops.GetPositions(session.ctx, &pb.PositionsRequest{
		AccountId: accountID,
	})
	if err != nil {
		return nil, fmt.Errorf("get positions: %w", err)
	}

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
		pos := parsePortfolioPosition(session.ctx, p, instruments)
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

	if snapshot.TotalValue == 0 {
		for _, p := range snapshot.Positions {
			snapshot.TotalValue += p.TotalValue
		}
	}

	return snapshot, nil
}

func (c *Client) GetAccounts(ctx context.Context, token string) ([]AccountInfo, error) {
	session, err := c.dial(ctx, token)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	resp, err := pb.NewUsersServiceClient(session.conn).GetAccounts(session.ctx, &pb.GetAccountsRequest{})
	if err != nil {
		return nil, fmt.Errorf("get accounts: %w", err)
	}

	var result []AccountInfo
	for _, a := range resp.GetAccounts() {
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

func (c *Client) dial(ctx context.Context, token string) (*grpcSession, error) {
	token = sanitizeToken(token)
	if token == "" {
		return nil, fmt.Errorf("empty API token")
	}

	endpoint := normalizeEndpoint(c.endpoint)

	conn, err := c.grpcDial(endpoint, token, c.tlsInsecure)
	if err != nil && !c.tlsInsecure && isTLSVerifyError(err) {
		log.Printf("[tinkoff] TLS verify failed (%v), retrying with InsecureSkipVerify", err)
		conn, err = c.grpcDial(endpoint, token, true)
	}
	if err != nil {
		return nil, fmt.Errorf("create tinkoff client: %w", err)
	}

	rpcCtx := metadata.AppendToOutgoingContext(ctx, "x-app-name", appName)
	return &grpcSession{conn: conn, ctx: rpcCtx}, nil
}

func (c *Client) grpcDial(endpoint, token string, insecureSkipVerify bool) (*grpc.ClientConn, error) {
	tlsCfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: insecureSkipVerify, //nolint:gosec // optional fallback for broken CA bundles
	}

	return grpc.Dial(endpoint,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)),
		grpc.WithPerRPCCredentials(oauth.TokenSource{
			TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}),
		}),
	)
}

func isTLSVerifyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "certificate") ||
		strings.Contains(msg, "x509") ||
		strings.Contains(msg, "authentication handshake failed")
}

func firstAccountID(ctx context.Context, users pb.UsersServiceClient) (string, error) {
	resp, err := users.GetAccounts(ctx, &pb.GetAccountsRequest{})
	if err != nil {
		return "", fmt.Errorf("get accounts: %w", err)
	}
	accounts := resp.GetAccounts()
	if len(accounts) == 0 {
		return "", fmt.Errorf("no accounts found")
	}
	return accounts[0].GetId(), nil
}

func parsePortfolioPosition(ctx context.Context, p *pb.PortfolioPosition, instruments pb.InstrumentsServiceClient) analytics.Position {
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
		if share, err := instruments.ShareBy(ctx, &pb.InstrumentRequest{
			IdType: pb.InstrumentIdType_INSTRUMENT_ID_TYPE_FIGI,
			Id:     figi,
		}); err == nil && share.GetInstrument() != nil {
			inst := share.GetInstrument()
			pos.Ticker = inst.GetTicker()
			pos.Name = inst.GetName()
			pos.AssetType = analytics.AssetShare
			return pos
		}
		if bond, err := instruments.BondBy(ctx, &pb.InstrumentRequest{
			IdType: pb.InstrumentIdType_INSTRUMENT_ID_TYPE_FIGI,
			Id:     figi,
		}); err == nil && bond.GetInstrument() != nil {
			inst := bond.GetInstrument()
			pos.Ticker = inst.GetTicker()
			pos.Name = inst.GetName()
			pos.AssetType = analytics.AssetBond
			return pos
		}
		if etf, err := instruments.EtfBy(ctx, &pb.InstrumentRequest{
			IdType: pb.InstrumentIdType_INSTRUMENT_ID_TYPE_FIGI,
			Id:     figi,
		}); err == nil && etf.GetInstrument() != nil {
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

func normalizeEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")

	if endpoint == "" || looksLikeToken(endpoint) || !strings.Contains(endpoint, ":") {
		return defaultEndpoint
	}
	return endpoint
}

func sanitizeToken(token string) string {
	token = strings.TrimSpace(token)
	token = strings.Trim(token, `"'`)
	token = strings.TrimPrefix(token, "Bearer ")
	return strings.TrimSpace(token)
}

func looksLikeToken(s string) bool {
	return strings.HasPrefix(s, "t.") && len(s) > 40 && !strings.Contains(s, ":")
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
