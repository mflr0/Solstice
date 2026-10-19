// Package jirafixture builds a synthetic Jira Data Center database, with the
// real Jira schema and realistic data, to test Solstice's importer.
//
// The schema is read at runtime from the OFBiz entity model shipped in a
// local Jira distribution, so tables and column types are exactly the ones
// Jira itself creates on PostgreSQL or MySQL. Nothing from the distribution
// is copied into this repository.
package jirafixture

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"solstice/internal/jiradb"
)

// EntityDefsDir is where a Jira distribution keeps its OFBiz entity model.
const EntityDefsDir = "atlassian-jira/WEB-INF/classes/entitydefs"

// Model is the OFBiz entity model of a Jira distribution.
type Model struct {
	Entities []*Entity
	byTable  map[string]*Entity // keyed by lower-case table name
}

// Entity is one table of the Jira schema.
type Entity struct {
	Name   string
	Table  string // as declared, e.g. "jiraissue" or "OS_CURRENTSTEP"
	Fields []Field
	PK     []string // column names
}

// Field is one column of an entity.
type Field struct {
	Name   string // OFBiz field name, e.g. "workflowId"
	Column string // database column, e.g. "WORKFLOW_ID"
	Type   string // OFBiz field type, e.g. "numeric"
}

// Entity returns the entity stored in table (case-insensitive), or nil.
func (m *Model) Entity(table string) *Entity {
	return m.byTable[strings.ToLower(table)]
}

// Column returns the declared column matching name (case-insensitive).
func (e *Entity) Column(name string) (Field, bool) {
	for _, f := range e.Fields {
		if strings.EqualFold(f.Column, name) {
			return f, true
		}
	}
	return Field{}, false
}

type xmlEntityModel struct {
	Entities []struct {
		Name   string `xml:"entity-name,attr"`
		Table  string `xml:"table-name,attr"`
		Fields []struct {
			Name    string `xml:"name,attr"`
			ColName string `xml:"col-name,attr"`
			Type    string `xml:"type,attr"`
		} `xml:"field"`
		PrimKeys []struct {
			Field string `xml:"field,attr"`
		} `xml:"prim-key"`
	} `xml:"entity"` // <view-entity> elements are skipped: they are not tables
}

// LoadModel parses entitymodel.xml.
func LoadModel(path string) (*Model, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	m, err := ParseModel(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

// ParseModel parses the content of an OFBiz entitymodel.xml.
func ParseModel(r io.Reader) (*Model, error) {
	var doc xmlEntityModel
	dec := xml.NewDecoder(r)
	dec.Strict = false
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	m := &Model{byTable: map[string]*Entity{}}
	for _, xe := range doc.Entities {
		e := &Entity{Name: xe.Name, Table: xe.Table}
		if e.Table == "" {
			e.Table = JavaNameToDBName(xe.Name)
		}
		byField := map[string]string{}
		for _, xf := range xe.Fields {
			col := xf.ColName
			if col == "" {
				col = JavaNameToDBName(xf.Name)
			}
			e.Fields = append(e.Fields, Field{Name: xf.Name, Column: col, Type: xf.Type})
			byField[xf.Name] = col
		}
		for _, pk := range xe.PrimKeys {
			col, ok := byField[pk.Field]
			if !ok {
				return nil, fmt.Errorf("entity %s: primary key on unknown field %q", e.Name, pk.Field)
			}
			e.PK = append(e.PK, col)
		}
		key := strings.ToLower(e.Table)
		if _, dup := m.byTable[key]; dup {
			return nil, fmt.Errorf("table %s is declared twice", e.Table)
		}
		m.byTable[key] = e
		m.Entities = append(m.Entities, e)
	}
	if len(m.Entities) == 0 {
		return nil, fmt.Errorf("no <entity> found")
	}
	return m, nil
}

// JavaNameToDBName applies OFBiz's naming rule for columns declared without
// a col-name: every upper-case letter starts a new underscore-separated word,
// and the result is upper-cased (workflowId → WORKFLOW_ID).
func JavaNameToDBName(name string) string {
	var b strings.Builder
	for i, r := range name {
		if i > 0 && unicode.IsUpper(r) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToUpper(r))
	}
	return b.String()
}

// FieldTypesFile is the OFBiz field type mapping Jira uses for an engine.
func FieldTypesFile(eng jiradb.Engine) string {
	if eng == jiradb.MySQL {
		return "fieldtype-mysql.xml"
	}
	return "fieldtype-postgres72.xml"
}

// LoadFieldTypes reads an OFBiz fieldtype-*.xml: field type → SQL type.
func LoadFieldTypes(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var doc struct {
		Defs []struct {
			Type    string `xml:"type,attr"`
			SQLType string `xml:"sql-type,attr"`
		} `xml:"field-type-def"`
	}
	dec := xml.NewDecoder(f)
	dec.Strict = false
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	types := make(map[string]string, len(doc.Defs))
	for _, d := range doc.Defs {
		types[d.Type] = d.SQLType
	}
	return types, nil
}

// LoadDistribution reads the entity model and the field types for eng from a
// Jira distribution directory.
func LoadDistribution(dist string, eng jiradb.Engine) (*Model, map[string]string, error) {
	dir := filepath.Join(dist, EntityDefsDir)
	m, err := LoadModel(filepath.Join(dir, "entitymodel.xml"))
	if err != nil {
		return nil, nil, err
	}
	types, err := LoadFieldTypes(filepath.Join(dir, FieldTypesFile(eng)))
	if err != nil {
		return nil, nil, err
	}
	return m, types, nil
}

// DDL returns the statements that drop and recreate every entity table.
// Only primary keys are created: Jira's secondary indexes do not matter for
// a fixture.
func (m *Model) DDL(eng jiradb.Engine, types map[string]string) ([]string, error) {
	stmts := make([]string, 0, 2*len(m.Entities))
	for _, e := range m.Entities {
		t := eng.QuoteFolded(e.Table)
		if eng == jiradb.MySQL {
			stmts = append(stmts, "DROP TABLE IF EXISTS "+t)
		} else {
			stmts = append(stmts, "DROP TABLE IF EXISTS "+t+" CASCADE")
		}

		pk := map[string]bool{}
		for _, c := range e.PK {
			pk[c] = true
		}
		var b strings.Builder
		b.WriteString("CREATE TABLE " + t + " (")
		for i, f := range e.Fields {
			sqlType, ok := types[f.Type]
			if !ok {
				return nil, fmt.Errorf("%s.%s: unknown field type %q", e.Table, f.Column, f.Type)
			}
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(eng.QuoteFolded(f.Column) + " " + sqlType)
			if pk[f.Column] {
				b.WriteString(" NOT NULL")
			}
		}
		if len(e.PK) > 0 {
			cols := make([]string, len(e.PK))
			for i, c := range e.PK {
				cols[i] = eng.QuoteFolded(c)
			}
			if eng == jiradb.MySQL {
				b.WriteString(", PRIMARY KEY (" + strings.Join(cols, ", ") + ")")
			} else {
				name := eng.QuoteFolded("pk_" + e.Table)
				b.WriteString(", CONSTRAINT " + name + " PRIMARY KEY (" + strings.Join(cols, ", ") + ")")
			}
		}
		b.WriteString(")")
		if eng == jiradb.MySQL {
			b.WriteString(" ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin")
		}
		stmts = append(stmts, b.String())
	}
	return stmts, nil
}
