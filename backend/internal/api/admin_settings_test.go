package api

import (
	"bytes"
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestGeneralFailedReadIsNotCached(t *testing.T) {
	var logs bytes.Buffer
	s := &Server{db: sql.OpenDB(failingDB{}), log: slog.New(slog.NewTextHandler(&logs, nil))}
	g := s.general(context.Background())
	if g.PasswordMinLength != defaultPasswordMin || g.SessionDays != defaultSessionDays {
		t.Errorf("fallback = %d characters, %d days; want the defaults", g.PasswordMinLength, g.SessionDays)
	}
	if s.adm.gen != nil {
		t.Error("the fallback of a failed read is cached")
	}
	if !strings.Contains(logs.String(), "reading the instance settings failed") {
		t.Errorf("no warning logged: %q", logs.String())
	}
}

func TestGeneralFailedReadKeepsTheCachedSettings(t *testing.T) {
	s := &Server{db: sql.OpenDB(failingDB{}), log: slog.New(slog.DiscardHandler)}
	cached := &generalSettings{PasswordMinLength: 20}
	s.adm.gen, s.adm.genAt = cached, time.Now().Add(-time.Hour)
	if g := s.general(context.Background()); g != cached {
		t.Errorf("general = %+v, want the cached settings", g)
	}
}
