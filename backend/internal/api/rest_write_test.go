package api

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"testing"

	"solstice/internal/i18n"
)

// failingDB is a database whose every query fails, for code that reads a
// value before it reads the database.
type failingDB struct{}

func (failingDB) Connect(context.Context) (driver.Conn, error) { return failingDB{}, nil }
func (failingDB) Driver() driver.Driver                        { return nil }
func (failingDB) Prepare(string) (driver.Stmt, error)          { return nil, errors.New("no database") }
func (failingDB) Close() error                                 { return nil }
func (failingDB) Begin() (driver.Tx, error)                    { return nil, errors.New("no database") }

// TestTranslateCustomTakesSettableKinds: translateCustom does not refuse any
// kind settableKind accepts as one it cannot set. The value given is not valid
// for any kind: what matters is the refusal each kind gives.
func TestTranslateCustomTakesSettableKinds(t *testing.T) {
	s := &Server{db: sql.OpenDB(failingDB{})}
	for _, kind := range settableKinds {
		p := &editProject{customs: []customFieldDef{{ID: 1, Name: "Champ", Kind: kind}}}
		err := s.translateCustom(context.Background(), p, &jiraEdit{ui: fields{}}, "customfield_1", json.RawMessage(`{`))
		var he *httpError
		if errors.As(err, &he) && he.msg.Key == i18n.FieldNotSettable {
			t.Errorf("settableKind takes %s, translateCustom does not", kind)
		}
	}
}
