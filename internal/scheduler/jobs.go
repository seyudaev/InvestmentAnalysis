package scheduler

import (
	"context"
	"fmt"
	"log"

	"github.com/robfig/cron/v3"
	"github.com/seyud/investment-analysis/internal/analytics"
	"github.com/seyud/investment-analysis/internal/broker/tinkoff"
	"github.com/seyud/investment-analysis/internal/storage"
	tele "gopkg.in/telebot.v4"
)

type Scheduler struct {
	cron    *cron.Cron
	store   *storage.Store
	crypto  *storage.Encryptor
	tinkoff *tinkoff.Client
	tb      *tele.Bot
}

func New(store *storage.Store, crypto *storage.Encryptor, tinkoffClient *tinkoff.Client, tb *tele.Bot) *Scheduler {
	return &Scheduler{
		cron:    cron.New(),
		store:   store,
		crypto:  crypto,
		tinkoff: tinkoffClient,
		tb:      tb,
	}
}

func (s *Scheduler) Start(digestHour int) {
	spec := fmt.Sprintf("0 %d * * *", digestHour)
	_, err := s.cron.AddFunc(spec, s.sendDailyDigests)
	if err != nil {
		log.Printf("scheduler: failed to add cron job: %v", err)
		return
	}
	s.cron.Start()
	log.Printf("Scheduler started: daily digest at %02d:00", digestHour)
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

func (s *Scheduler) sendDailyDigests() {
	log.Println("Sending daily digests...")
}

func (s *Scheduler) SendDigestToUser(telegramID int64, userID int64) error {
	bt, err := s.store.GetBrokerToken(userID, "tinkoff")
	if err != nil {
		return err
	}

	token, err := s.crypto.Decrypt(bt.TokenEnc)
	if err != nil {
		return err
	}

	snapshot, err := s.tinkoff.GetPortfolio(context.Background(), token, bt.AccountID)
	if err != nil {
		return err
	}

	allocs, _ := s.store.GetTargetAllocations(userID)
	targets := analytics.DefaultTargets()
	if len(allocs) > 0 {
		targets = make(map[analytics.AssetType]float64)
		for _, a := range allocs {
			targets[analytics.AssetType(a.AssetType)] = a.TargetPct
		}
	}

	analysis := analytics.AnalyzePortfolio(*snapshot, targets)
	report := analytics.FormatPortfolioReport(analysis)
	signals := analytics.FormatSignalsReport(analysis.Signals)

	_, err = s.tb.Send(&tele.User{ID: telegramID}, report+"\n"+signals, tele.ModeMarkdown)
	return err
}
