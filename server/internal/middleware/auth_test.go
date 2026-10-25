package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("auth service offline")
}

func TestHandyAuth(t *testing.T) {
	const identity = "123e4567-e89b-12d3-a456-426614174000"
	tests := []struct {
		name      string
		token     string
		status    int
		body      string
		wantCode  int
		wantError string
	}{
		{"missing authorization", "", http.StatusOK, `{"valid":true,"user_id":"` + identity + `"}`, http.StatusUnauthorized, "authentication required"},
		{"valid session", "Bearer session-token", http.StatusOK, `{"valid":true,"user_id":"` + identity + `"}`, http.StatusNoContent, ""},
		{"upstream unauthorized", "Bearer expired", http.StatusUnauthorized, `{}`, http.StatusUnauthorized, "invalid or expired session"},
		{"upstream error", "Bearer token", http.StatusInternalServerError, `{}`, http.StatusUnauthorized, "invalid or expired session"},
		{"malformed response", "Bearer token", http.StatusOK, `{`, http.StatusUnauthorized, "invalid or expired session"},
		{"invalid session", "Bearer token", http.StatusOK, `{"valid":false,"user_id":"` + identity + `"}`, http.StatusUnauthorized, "invalid or expired session"},
		{"missing validity", "Bearer token", http.StatusOK, `{"user_id":"` + identity + `"}`, http.StatusUnauthorized, "invalid or expired session"},
		{"invalid identity", "Bearer token", http.StatusOK, `{"valid":true,"user_id":"not-a-uuid"}`, http.StatusUnauthorized, "invalid user identity"},
		{"missing identity", "Bearer token", http.StatusOK, `{"valid":true}`, http.StatusUnauthorized, "invalid user identity"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodGet || r.URL.Path != "/api/auth/validate" {
					t.Errorf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
				}
				if got := r.Header.Get("Authorization"); got != tt.token {
					t.Errorf("forwarded Authorization = %q, want %q", got, tt.token)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer upstream.Close()

			router := gin.New()
			called := false
			router.GET("/protected", HandyAuth(upstream.URL+"///", upstream.Client()), func(c *gin.Context) {
				called = true
				got, exists := c.Get(UserIDKey)
				if !exists || got != uuid.MustParse(identity) {
					t.Errorf("context user identity = %v (exists %v), want UUID %s", got, exists, identity)
				}
				c.Status(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.token != "" {
				request.Header.Set("Authorization", tt.token)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, tt.wantCode, response.Body)
			}
			if called != (tt.wantCode == http.StatusNoContent) {
				t.Errorf("protected handler called = %v", called)
			}
			wantCalls := 1
			if tt.token == "" {
				wantCalls = 0
			}
			if calls != wantCalls {
				t.Errorf("upstream calls = %d, want %d", calls, wantCalls)
			}
			if tt.wantError != "" {
				assertAuthError(t, response, tt.wantError)
			}
		})
	}
}

func TestHandyAuthServiceFailures(t *testing.T) {
	for _, tt := range []struct {
		name      string
		url       string
		client    *http.Client
		wantCode  int
		wantError string
	}{
		{"invalid configuration", "://invalid", nil, http.StatusInternalServerError, "authentication configuration error"},
		{"transport failure", "http://auth.invalid", &http.Client{Transport: failingTransport{}}, http.StatusServiceUnavailable, "authentication service unavailable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/protected", HandyAuth(tt.url, tt.client), func(c *gin.Context) {
				t.Error("protected handler called after authentication failure")
			})
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			request.Header.Set("Authorization", "Bearer token")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantCode)
			}
			assertAuthError(t, response, tt.wantError)
		})
	}
}

func assertAuthError(t *testing.T, response *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error != want {
		t.Errorf("error = %q, want %q", body.Error, want)
	}
}
