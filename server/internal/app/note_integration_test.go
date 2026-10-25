package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mhandyalf/handy-notion/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// This exercises real HTTP, auth middleware, handlers, and concurrent PostgreSQL
// connections. Only the external auth service is stubbed. Each run owns a schema.
func TestPostgresNoteConcurrency(t *testing.T) {
	dsn := os.Getenv("NOTE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set NOTE_TEST_DATABASE_URL to a disposable PostgreSQL database URL")
	}
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatal("NOTE_TEST_DATABASE_URL must be a postgres:// or postgresql:// URL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal("connect to integration database:", err)
	}
	admin, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	schema := "note_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := db.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
	})
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	isolated, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := isolated.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = pool.Close() })
	if err := isolated.AutoMigrate(&model.Note{}); err != nil {
		t.Fatal(err)
	}
	owner, other := uuid.NewString(), uuid.NewString()
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/validate" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		user := owner
		switch r.Header.Get("Authorization") {
		case "Bearer tab-a", "Bearer tab-b":
		case "Bearer other-user":
			user = other
		default:
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"valid": true, "user_id": user})
	}))
	t.Cleanup(auth.Close)
	server := httptest.NewServer(NewRouter(isolated, auth.URL, []string{"http://localhost:5174"}))
	t.Cleanup(server.Close)
	// Separate transports model two independent clients sharing one user.
	a := &http.Client{Transport: &http.Transport{}, Timeout: 10 * time.Second}
	b := &http.Client{Transport: &http.Transport{}, Timeout: 10 * time.Second}
	t.Cleanup(a.CloseIdleConnections)
	t.Cleanup(b.CloseIdleConnections)
	type result struct {
		status int
		body   []byte
		err    error
	}
	request := func(client *http.Client, token, method, path string, body any) result {
		var data []byte
		var err error
		if body != nil {
			data, err = json.Marshal(body)
			if err != nil {
				return result{err: err}
			}
		}
		req, e := http.NewRequest(method, server.URL+path, bytes.NewReader(data))
		if e != nil {
			return result{err: e}
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		res, e := client.Do(req)
		if e != nil {
			return result{err: e}
		}
		defer res.Body.Close()
		data, e = io.ReadAll(res.Body)
		return result{res.StatusCode, data, e}
	}
	check := func(t *testing.T, res result, status int) {
		t.Helper()
		if res.err != nil || res.status != status {
			t.Fatalf("response = %d %s (%v), want %d", res.status, res.body, res.err, status)
		}
		if status == http.StatusConflict {
			var problem struct{ Code string }
			if json.Unmarshal(res.body, &problem) != nil || problem.Code != "note_conflict" {
				t.Fatalf("missing conflict code: %s", res.body)
			}
		}
	}
	decode := func(t *testing.T, res result) model.Note {
		t.Helper()
		var note model.Note
		if err := json.Unmarshal(res.body, &note); err != nil {
			t.Fatal(err)
		}
		return note
	}
	create := func(t *testing.T) model.Note {
		t.Helper()
		res := request(a, "tab-a", "POST", "/api/notes", map[string]any{"title": "original", "content": []any{}})
		check(t, res, http.StatusCreated)
		return decode(t, res)
	}
	version := func(note model.Note) string { return note.UpdatedAt.Format(time.RFC3339Nano) }
	update := func(note model.Note, title string) map[string]any {
		return map[string]any{"title": title, "expected_updated_at": version(note)}
	}
	t.Run("stale clients and database timestamp precision", func(t *testing.T) {
		original := create(t)
		path := "/api/notes/" + original.ID.String()
		read := request(b, "tab-b", "GET", path, nil)
		check(t, read, 200)
		if !decode(t, read).UpdatedAt.Equal(original.UpdatedAt) || original.UpdatedAt.Nanosecond()%1000 != 0 {
			t.Fatal("create returned a version that differs from PostgreSQL precision")
		}
		check(t, request(a, "tab-a", "PUT", path, map[string]any{"title": "no precondition"}), 428)
		check(t, request(a, "tab-a", "DELETE", path, nil), 428)
		check(t, request(a, "tab-a", "PUT", path, map[string]any{"title": "invalid", "expected_updated_at": "invalid"}), 400)
		check(t, request(a, "tab-a", "DELETE", path+"?expected_updated_at=invalid", nil), 400)
		check(t, request(b, "other-user", "PUT", path, update(original, "unowned")), 409)
		check(t, request(b, "other-user", "DELETE", path+"?expected_updated_at="+url.QueryEscape(version(original)), nil), 409)
		saved := request(a, "tab-a", "PUT", path, update(original, "winner"))
		check(t, saved, 200)
		latest := decode(t, saved)
		if latest.UpdatedAt.Sub(original.UpdatedAt) < time.Microsecond {
			t.Fatal("timestamp failed to advance by at least one microsecond")
		}
		check(t, request(b, "tab-b", "PUT", path, update(original, "stale")), 409)
		check(t, request(b, "tab-b", "DELETE", path+"?expected_updated_at="+url.QueryEscape(version(original)), nil), 409)
		read = request(b, "tab-b", "GET", path, nil)
		check(t, read, 200)
		if got := decode(t, read); got.Title != "winner" || !got.UpdatedAt.Equal(latest.UpdatedAt) {
			t.Fatal("rejected mutation changed the winning row")
		}
		// Even if the stored version is ahead of the DB clock, it must advance.
		future := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
		if err := isolated.Model(&model.Note{}).Where("id = ?", original.ID).UpdateColumn("updated_at", future).Error; err != nil {
			t.Fatal(err)
		}
		latest.UpdatedAt = future
		saved = request(a, "tab-a", "PUT", path, update(latest, "future clock"))
		check(t, saved, 200)
		latest = decode(t, saved)
		if !latest.UpdatedAt.Equal(future.Add(time.Microsecond)) {
			t.Fatal("future timestamp did not advance by exactly one microsecond")
		}
		check(t, request(a, "tab-a", "DELETE", path+"?expected_updated_at="+url.QueryEscape(version(latest)), nil), 204)
		check(t, request(b, "tab-b", "PUT", path, update(latest, "resurrect")), 409)
		check(t, request(b, "tab-b", "DELETE", path+"?expected_updated_at="+url.QueryEscape(version(latest)), nil), 409)
		check(t, request(a, "tab-a", "GET", path, nil), 404)
	})
	t.Run("archive invalidates an editing tab", func(t *testing.T) {
		note := create(t)
		path := "/api/notes/" + note.ID.String()
		archived := request(a, "tab-a", "PUT", path, map[string]any{
			"is_archived": true, "expected_updated_at": version(note),
		})
		check(t, archived, 200)
		check(t, request(b, "tab-b", "PUT", path, update(note, "stale edit")), 409)
		read := request(b, "tab-b", "GET", path, nil)
		check(t, read, 200)
		stored := decode(t, read)
		if !stored.IsArchived || stored.Title != note.Title || !stored.UpdatedAt.Equal(decode(t, archived).UpdatedAt) {
			t.Fatal("stale edit modified the archived note")
		}
	})
	for _, deleteSecond := range []bool{false, true} {
		t.Run(fmt.Sprintf("concurrent mutations delete=%v", deleteSecond), func(t *testing.T) {
			for attempt := 0; attempt < 10; attempt++ {
				note := create(t)
				path := "/api/notes/" + note.ID.String()
				start := make(chan struct{})
				results := make(chan result, 2)
				go func() {
					<-start
					results <- request(a, "tab-a", "PUT", path, update(note, "tab a"))
				}()
				go func() {
					<-start
					if deleteSecond {
						results <- request(b, "tab-b", "DELETE", path+"?expected_updated_at="+url.QueryEscape(version(note)), nil)
					} else {
						results <- request(b, "tab-b", "PUT", path, update(note, "tab b"))
					}
				}()
				close(start)
				first, second := <-results, <-results
				if first.status == 409 {
					first, second = second, first
				}
				check(t, second, 409)
				if first.status != 200 && !(deleteSecond && first.status == 204) {
					t.Fatalf("no unique winning mutation: %+v", first)
				}
				check(t, first, first.status)
				read := request(a, "tab-a", "GET", path, nil)
				if first.status == 204 {
					check(t, read, 404)
				} else {
					check(t, read, 200)
					winner, stored := decode(t, first), decode(t, read)
					if winner.Title != stored.Title || !winner.UpdatedAt.Equal(stored.UpdatedAt) || winner.UpdatedAt.Sub(note.UpdatedAt) < time.Microsecond {
						t.Fatal("winning response does not match the stored advancing version")
					}
				}
			}
		})
	}
}
