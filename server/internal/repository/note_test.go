package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mhandyalf/handy-notion/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// A per-test connector avoids global registration and any external database.
// Script transaction boundaries too, so extra reads/writes or missing rollbacks fail.
type noteSQLStep struct {
	op       string
	check    func(string, []driver.NamedValue)
	columns  []string
	rows     [][]driver.Value
	affected int64
	err      error
}
type noteSQLDriver struct {
	t     *testing.T
	steps []noteSQLStep
	inTx  bool
}

func (d *noteSQLDriver) next(op string) noteSQLStep {
	d.t.Helper()
	if len(d.steps) == 0 {
		d.t.Fatalf("unexpected SQL operation %s", op)
	}
	s := d.steps[0]
	d.steps = d.steps[1:]
	if s.op != op {
		d.t.Fatalf("SQL operation = %s, want %s", op, s.op)
	}
	return s
}
func (d *noteSQLDriver) Connect(context.Context) (driver.Conn, error) { return &noteSQLConn{d}, nil }
func (d *noteSQLDriver) Driver() driver.Driver                        { return d }
func (d *noteSQLDriver) Open(string) (driver.Conn, error)             { return &noteSQLConn{d}, nil }

type noteSQLConn struct{ d *noteSQLDriver }

func (c *noteSQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}
func (c *noteSQLConn) Close() error { return nil }
func (c *noteSQLConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *noteSQLConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	s := c.d.next("begin")
	if c.d.inTx {
		c.d.t.Fatal("nested transaction")
	}
	if s.err != nil {
		return nil, s.err
	}
	c.d.inTx = true
	return &noteSQLTx{c.d}, nil
}
func (c *noteSQLConn) write(op, query string, args []driver.NamedValue) noteSQLStep {
	s := c.d.next(op)
	if !c.d.inTx {
		c.d.t.Fatal("write outside transaction")
	}
	if s.check != nil {
		s.check(query, args)
	}
	return s
}
func (c *noteSQLConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	s := c.write("query", q, args)
	if s.err != nil {
		return nil, s.err
	}
	return &noteSQLRows{columns: s.columns, rows: s.rows}, nil
}
func (c *noteSQLConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	s := c.write("exec", q, args)
	return driver.RowsAffected(s.affected), s.err
}

type noteSQLTx struct{ d *noteSQLDriver }

func (tx *noteSQLTx) finish(op string) error {
	s := tx.d.next(op)
	if !tx.d.inTx {
		tx.d.t.Fatal("transaction already finished")
	}
	tx.d.inTx = false
	return s.err
}
func (tx *noteSQLTx) Commit() error   { return tx.finish("commit") }
func (tx *noteSQLTx) Rollback() error { return tx.finish("rollback") }

type noteSQLRows struct {
	columns []string
	rows    [][]driver.Value
}

func (r *noteSQLRows) Columns() []string { return r.columns }
func (r *noteSQLRows) Close() error      { return nil }
func (r *noteSQLRows) Next(dest []driver.Value) error {
	if len(r.rows) == 0 {
		return io.EOF
	}
	copy(dest, r.rows[0])
	r.rows = r.rows[1:]
	return nil
}
func newNoteTestRepository(t *testing.T, steps ...noteSQLStep) *NoteRepository {
	t.Helper()
	d := &noteSQLDriver{t: t, steps: steps}
	db := sql.OpenDB(d)
	db.SetMaxOpenConns(1)
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{
		DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if len(d.steps) != 0 {
			t.Errorf("%d unconsumed SQL steps; next = %s", len(d.steps), d.steps[0].op)
		}
		if d.inTx {
			t.Error("transaction left open")
		}
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return NewNoteRepository(gdb)
}

var (
	noteTestUser    = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	noteTestID      = uuid.MustParse("22222222-2222-4222-8222-222222222222")
	noteTestVersion = time.Date(2035, 2, 3, 4, 5, 6, 123456000, time.UTC)
)

// Check actual SQL and bound values rather than interpolated logger output.
func noteMutationCheck(t *testing.T, update bool, expected time.Time) func(string, []driver.NamedValue) {
	t.Helper()
	return func(q string, args []driver.NamedValue) {
		t.Helper()
		if !strings.HasPrefix(q, `UPDATE "notes" SET `) {
			t.Errorf("not a single UPDATE: %s", q)
		}
		where := strings.Index(q, " WHERE ")
		if where < 0 {
			t.Errorf("missing WHERE: %s", q)
			return
		}
		predicates := q[where:]
		for _, p := range []struct {
			field string
			value any
		}{{"id", noteTestID.String()}, {"user_id", noteTestUser.String()}, {"updated_at", expected}} {
			match := regexp.MustCompile(`\b` + p.field + ` = \$(\d+)`).FindStringSubmatch(predicates)
			if match == nil {
				t.Errorf("missing atomic %s predicate: %s", p.field, q)
				continue
			}
			n, _ := strconv.Atoi(match[1])
			if n < 1 || n > len(args) {
				t.Errorf("invalid placeholder %s", match[0])
				continue
			}
			if !reflect.DeepEqual(args[n-1].Value, p.value) {
				t.Errorf("%s binding = %#v, want %#v", p.field, args[n-1].Value, p.value)
			}
		}
		if !strings.Contains(predicates, `"notes"."deleted_at" IS NULL`) {
			t.Errorf("missing soft-delete scope: %s", q)
		}
		if strings.Contains(predicates, " OR ") {
			t.Errorf("predicates must all match: %s", q)
		}
		if update {
			expression := `"updated_at"=GREATEST(date_trunc('microseconds', clock_timestamp()), updated_at + INTERVAL '1 microsecond')`
			if !strings.Contains(q[:where], expression) {
				t.Errorf("missing microsecond-monotonic database expression: %s", q)
			}
			if !strings.HasSuffix(q, ` RETURNING *`) {
				t.Errorf("must return exact updated row: %s", q)
			}
		} else if !strings.Contains(q[:where], `"deleted_at"=`) {
			t.Errorf("delete must be soft: %s", q)
		}
	}
}

func noteReturnedRow() (model.Note, noteSQLStep) {
	n := model.Note{ID: noteTestID, UserID: noteTestUser, Title: "database title", Content: model.JSON(`[{"type":"paragraph"}]`), Icon: "DB", IsFavorite: true, IsArchived: true, CreatedAt: noteTestVersion.Add(-time.Hour), UpdatedAt: noteTestVersion.Add(time.Microsecond)}
	return n, noteSQLStep{op: "query", columns: []string{"id", "user_id", "title", "content", "icon", "is_favorite", "is_archived", "created_at", "updated_at", "deleted_at"}, rows: [][]driver.Value{{n.ID.String(), n.UserID.String(), n.Title, []byte(n.Content), n.Icon, n.IsFavorite, n.IsArchived, n.CreatedAt, n.UpdatedAt, nil}}}
}

func TestNoteRepositoryUpdateReturnsDatabaseRow(t *testing.T) {
	want, row := noteReturnedRow()
	row.check = noteMutationCheck(t, true, noteTestVersion)
	repo := newNoteTestRepository(t, noteSQLStep{op: "begin"}, row, noteSQLStep{op: "commit"})
	// Caller timestamps must be overridden without mutating the input map.
	values := map[string]any{"title": "requested title", "is_favorite": false, "updated_at": time.Unix(1, 0)}
	before := map[string]any{}
	for k, v := range values {
		before[k] = v
	}
	got, err := repo.Update(noteTestUser, noteTestID, noteTestVersion, values)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, &want) {
		t.Errorf("returned note = %#v, want database row %#v", got, &want)
	}
	if !reflect.DeepEqual(values, before) {
		t.Errorf("caller values mutated: %#v", values)
	}
}

func TestNoteRepositoryDeleteCommitsSoftDelete(t *testing.T) {
	repo := newNoteTestRepository(t, noteSQLStep{op: "begin"}, noteSQLStep{op: "exec", affected: 1, check: noteMutationCheck(t, false, noteTestVersion)}, noteSQLStep{op: "commit"})
	if err := repo.Delete(noteTestUser, noteTestID, noteTestVersion); err != nil {
		t.Fatal(err)
	}
}

func TestNoteRepositoryMutationFailures(t *testing.T) {
	statementErr := errors.New("statement failed")
	commitErr := errors.New("commit failed")
	beginErr := errors.New("begin failed")
	for _, update := range []bool{true, false} {
		name := "Delete"
		if update {
			name = "Update"
		}
		for _, scenario := range []string{"zero rows", "statement error", "commit error", "begin error"} {
			t.Run(name+"/"+scenario, func(t *testing.T) {
				_, write := noteReturnedRow()
				if !update {
					write = noteSQLStep{op: "exec", affected: 1}
				}
				write.check = noteMutationCheck(t, update, noteTestVersion)
				begin := noteSQLStep{op: "begin"}
				end := noteSQLStep{op: "rollback"}
				wantErr := ErrNoteConflict
				switch scenario {
				case "zero rows":
					write.rows = nil
					write.affected = 0
				case "statement error":
					write.err = statementErr
					wantErr = statementErr
				case "commit error":
					end = noteSQLStep{op: "commit", err: commitErr}
					wantErr = commitErr
				case "begin error":
					begin.err = beginErr
					wantErr = beginErr
				}
				steps := []noteSQLStep{begin, write, end}
				if scenario == "begin error" {
					steps = steps[:1]
				}
				repo := newNoteTestRepository(t, steps...)
				var err error
				if update {
					var got *model.Note
					got, err = repo.Update(noteTestUser, noteTestID, noteTestVersion, map[string]any{"title": "changed"})
					if got != nil {
						t.Errorf("failed update returned a note: %#v", got)
					}
				} else {
					err = repo.Delete(noteTestUser, noteTestID, noteTestVersion)
				}
				if !errors.Is(err, wantErr) {
					t.Errorf("error = %v, want %v", err, wantErr)
				}
			})
		}
	}
}

func TestNoteRepositoryRejectsNonMicrosecondTokenWithoutSQL(t *testing.T) {
	// Both sub-microsecond offsets would lose information at PostgreSQL precision.
	for _, offset := range []time.Duration{time.Nanosecond, 999 * time.Nanosecond} {
		t.Run(offset.String(), func(t *testing.T) {
			repo := newNoteTestRepository(t) // Any transaction or SQL is unexpected.
			expected := noteTestVersion.Add(offset)
			got, err := repo.Update(noteTestUser, noteTestID, expected, map[string]any{"title": "changed"})
			if got != nil || !errors.Is(err, ErrNoteConflict) {
				t.Errorf("Update = (%#v, %v), want nil, conflict", got, err)
			}
			if err := repo.Delete(noteTestUser, noteTestID, expected); !errors.Is(err, ErrNoteConflict) {
				t.Errorf("Delete = %v, want conflict", err)
			}
		})
	}
}

func TestNoteRepositoryUpdateScanErrorRollsBack(t *testing.T) {
	_, row := noteReturnedRow()
	row.check = noteMutationCheck(t, true, noteTestVersion)
	row.rows[0][8] = "not a timestamp"
	repo := newNoteTestRepository(t, noteSQLStep{op: "begin"}, row, noteSQLStep{op: "rollback"})
	got, err := repo.Update(noteTestUser, noteTestID, noteTestVersion, nil)
	if got != nil || err == nil || errors.Is(err, ErrNoteConflict) {
		t.Errorf("Update = (%#v, %v), want nil, scan error", got, err)
	}
}

func TestNoteRepositoryUpdateNilValuesStillAdvancesVersion(t *testing.T) {
	want, row := noteReturnedRow()
	row.check = noteMutationCheck(t, true, noteTestVersion)
	repo := newNoteTestRepository(t, noteSQLStep{op: "begin"}, row, noteSQLStep{op: "commit"})
	got, err := repo.Update(noteTestUser, noteTestID, noteTestVersion, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, &want) {
		t.Errorf("Update = %#v, want %#v", got, &want)
	}
}
