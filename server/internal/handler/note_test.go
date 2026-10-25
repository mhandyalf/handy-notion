package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mhandyalf/handy-notion/internal/middleware"
	"github.com/mhandyalf/handy-notion/internal/model"
	"github.com/mhandyalf/handy-notion/internal/repository"
)

// Embedding a nil repository makes any unexpected call (including Get) panic.
// These tests intentionally omit recovery middleware so such calls fail the test.
type mutationNoteRepository struct {
	noteRepository
	update func(uuid.UUID, uuid.UUID, time.Time, map[string]any) (*model.Note, error)
	delete func(uuid.UUID, uuid.UUID, time.Time) error
}

func (r *mutationNoteRepository) Update(owner, id uuid.UUID, expected time.Time, values map[string]any) (*model.Note, error) {
	return r.update(owner, id, expected, values)
}

func (r *mutationNoteRepository) Delete(owner, id uuid.UUID, expected time.Time) error {
	return r.delete(owner, id, expected)
}

func noteMutationRequest(repo noteRepository, owner uuid.UUID, method, target, body string) *httptest.ResponseRecorder {
	h := &NoteHandler{notes: repo}
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(middleware.UserIDKey, owner) })
	router.PATCH("/notes/:id", h.Update)
	router.DELETE("/notes/:id", h.Delete)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}

func assertNoteError(t *testing.T, response *httptest.ResponseRecorder, status int, message, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, status, response.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	want := map[string]any{"error": message}
	if code != "" {
		want["code"] = code
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("response = %#v, want %#v", got, want)
	}
}

func TestNoteMutationPreconditions(t *testing.T) {
	owner, id := uuid.New(), uuid.New()
	for _, method := range []string{http.MethodPatch, http.MethodDelete} {
		for _, tc := range []struct {
			name, token string
			status      int
		}{
			{"missing", "", http.StatusPreconditionRequired},
			{"null", `null`, http.StatusBadRequest},
			{"number", `123`, http.StatusBadRequest},
			{"empty", `""`, http.StatusBadRequest},
			{"malformed", `"not-a-timestamp"`, http.StatusBadRequest},
			{"date only", `"2025-01-02"`, http.StatusBadRequest},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				target, body := "/notes/"+id.String(), `{"title":"changed"}`
				message := "expected_updated_at is required"
				if tc.token != "" {
					message = "expected_updated_at must be an RFC3339Nano timestamp"
					if method == http.MethodPatch {
						body = `{"title":"changed","expected_updated_at":` + tc.token + `}`
						if tc.name == "null" || tc.name == "number" {
							message = "invalid expected_updated_at"
						}
					} else {
						target += "?expected_updated_at=" + url.QueryEscape(strings.Trim(tc.token, `"`))
					}
				}
				response := noteMutationRequest(&mutationNoteRepository{}, owner, method, target, body)
				assertNoteError(t, response, tc.status, message, "")
			})
		}
	}
}

func TestNoteMutationRepositoryResults(t *testing.T) {
	owner, id := uuid.New(), uuid.New()
	for _, token := range []string{"2025-02-03T04:05:06Z", "2025-02-03T04:05:06.123456789+05:30", "2025-02-03T04:05:06.987654321-07:00"} {
		for _, method := range []string{http.MethodPatch, http.MethodDelete} {
			for _, outcome := range []struct {
				name string
				err  error
			}{
				{"success", nil},
				{"conflict", repository.ErrNoteConflict},
				{"wrapped conflict", fmt.Errorf("mutation: %w", repository.ErrNoteConflict)},
				{"failure", errors.New("database unavailable")},
			} {
				t.Run(method+"/"+token+"/"+outcome.name, func(t *testing.T) {
					expected, err := time.Parse(time.RFC3339Nano, token)
					if err != nil {
						t.Fatal(err)
					}
					// Deliberately distinct from the submitted values: the response must
					// be the returned row, not a reconstruction or a second Get.
					row := &model.Note{ID: id, UserID: owner, Title: "persisted title", Icon: "stored icon",
						Content: model.JSON(`[{"id":"stored","type":"paragraph"}]`), IsFavorite: true,
						CreatedAt: expected.Add(-time.Hour), UpdatedAt: expected.Add(time.Second)}
					calls := 0
					check := func(gotOwner, gotID uuid.UUID, gotTime time.Time) {
						t.Helper()
						calls++
						if gotOwner != owner || gotID != id {
							t.Errorf("ownership IDs = (%s, %s), want (%s, %s)", gotOwner, gotID, owner, id)
						}
						if !gotTime.Equal(expected) || gotTime.Format(time.RFC3339Nano) != token {
							t.Errorf("timestamp = %s, want %s", gotTime.Format(time.RFC3339Nano), token)
						}
					}
					repo := &mutationNoteRepository{
						update: func(owner, id uuid.UUID, at time.Time, values map[string]any) (*model.Note, error) {
							check(owner, id, at)
							want := map[string]any{"title": "new title", "icon": "", "content": model.JSON(`[]`), "is_favorite": false, "is_archived": true}
							if !reflect.DeepEqual(values, want) {
								t.Errorf("updates = %#v, want %#v", values, want)
							}
							return row, outcome.err
						},
						delete: func(owner, id uuid.UUID, at time.Time) error {
							check(owner, id, at)
							return outcome.err
						},
					}
					target := "/notes/" + id.String()
					body := `{"expected_updated_at":"` + token + `","title":"  new title  ","icon":"  ","content":[],"is_favorite":false,"is_archived":true}`
					if method == http.MethodDelete {
						target += "?expected_updated_at=" + url.QueryEscape(token)
						body = ""
					}
					response := noteMutationRequest(repo, owner, method, target, body)
					if calls != 1 {
						t.Fatalf("repository calls = %d, want 1", calls)
					}
					switch {
					case errors.Is(outcome.err, repository.ErrNoteConflict):
						assertNoteError(t, response, http.StatusConflict, "note has changed; reload before retrying", "note_conflict")
					case outcome.err != nil:
						message := "failed to save note"
						if method == http.MethodDelete {
							message = "failed to delete note"
						}
						assertNoteError(t, response, http.StatusInternalServerError, message, "")
					case method == http.MethodDelete:
						if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
							t.Errorf("delete response = %d %q, want empty 204", response.Code, response.Body.String())
						}
					default:
						want, err := json.Marshal(row)
						if err != nil {
							t.Fatal(err)
						}
						if response.Code != http.StatusOK || response.Body.String() != string(want) {
							t.Errorf("update response = %d %s, want 200 %s", response.Code, response.Body.String(), want)
						}
					}
				})
			}
		}
	}
}

func TestNoteMutationValidation(t *testing.T) {
	owner, id := uuid.New(), uuid.New()
	const token = `"expected_updated_at":"2025-02-03T04:05:06Z"`
	for _, method := range []string{http.MethodPatch, http.MethodDelete} {
		t.Run(method+"/invalid id", func(t *testing.T) {
			response := noteMutationRequest(&mutationNoteRepository{}, owner, method, "/notes/not-a-uuid?expected_updated_at=2025-02-03T04:05:06Z", `{`+token+`,"title":"new"}`)
			assertNoteError(t, response, http.StatusBadRequest, "invalid note id", "")
		})
	}
	for _, tc := range []struct {
		name, body, message string
	}{
		{"malformed JSON", `{`, "invalid note"},
		{"wrong title type", `{` + token + `,"title":123}`, "invalid note"},
		{"wrong boolean type", `{` + token + `,"is_favorite":"yes"}`, "invalid note"},
		{"no fields", `{` + token + `}`, "no fields to update"},
		{"null fields", `{` + token + `,"title":null,"icon":null,"is_favorite":null,"is_archived":null}`, "no fields to update"},
		{"unknown fields", `{` + token + `,"user_id":"ignored"}`, "no fields to update"},
		{"null content", `{` + token + `,"content":null}`, "content must be a JSON array"},
		{"object content", `{` + token + `,"content":{}}`, "content must be a JSON array"},
		{"string content", `{` + token + `,"content":"[]"}`, "content must be a JSON array"},
	} {
		t.Run("PATCH/"+tc.name, func(t *testing.T) {
			response := noteMutationRequest(&mutationNoteRepository{}, owner, http.MethodPatch, "/notes/"+id.String(), tc.body)
			assertNoteError(t, response, http.StatusBadRequest, tc.message, "")
		})
	}
}

func TestNoteUpdateBlankTitleAndOmittedFields(t *testing.T) {
	owner, id := uuid.New(), uuid.New()
	calls := 0
	repo := &mutationNoteRepository{update: func(gotOwner, gotID uuid.UUID, expected time.Time, values map[string]any) (*model.Note, error) {
		calls++
		if gotOwner != owner || gotID != id || expected.Format(time.RFC3339Nano) != "2025-02-03T04:05:06Z" {
			t.Errorf("unexpected update arguments: %s %s %s", gotOwner, gotID, expected)
		}
		if want := map[string]any{"title": "Untitled"}; !reflect.DeepEqual(values, want) {
			t.Errorf("updates = %#v, want %#v (omitted fields must stay omitted)", values, want)
		}
		return &model.Note{ID: id, Title: "Untitled"}, nil
	}}
	response := noteMutationRequest(repo, owner, http.MethodPatch, "/notes/"+id.String(), `{"expected_updated_at":"2025-02-03T04:05:06Z","title":" \t "}`)
	if response.Code != http.StatusOK || calls != 1 {
		t.Errorf("status = %d, calls = %d; body: %s", response.Code, calls, response.Body.String())
	}
}

func TestValidBlocks(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{"empty array", `[]`, true},
		{"blocks", `[{"id":"block-1","type":"paragraph","content":"hello"}]`, true},
		{"whitespace", " \n\t[ ]\r\n", true},
		{"nested values", `[[1],null,true,"text"]`, true},
		{"missing", ``, false},
		{"whitespace only", " \n\t", false},
		{"null", `null`, false},
		{"object", `{}`, false},
		{"string", `"[]"`, false},
		{"number", `42`, false},
		{"boolean", `true`, false},
		{"unclosed array", `[`, false},
		{"trailing comma", `[{},]`, false},
		{"trailing data", `[] {}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validBlocks(json.RawMessage(tt.raw)); got != tt.want {
				t.Errorf("validBlocks(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
