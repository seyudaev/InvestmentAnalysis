package tinkoff

import (
	"log"
)

type sdkLogger struct{}

func newSDKLogger() *sdkLogger {
	return &sdkLogger{}
}

func (l *sdkLogger) Infof(template string, args ...any) {
	log.Printf("[tinkoff] "+template, args...)
}

func (l *sdkLogger) Errorf(template string, args ...any) {
	log.Printf("[tinkoff] ERROR: "+template, args...)
}

func (l *sdkLogger) Fatalf(template string, args ...any) {
	log.Fatalf("[tinkoff] FATAL: "+template, args...)
}
