package kitestream

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	srv := NewServer("/tmp", "/tmp", 0, "test-token")
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("health status = %d, want 200", w.Code)
	}
	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("health status = %q, want %q", body["status"], "ok")
	}
}

func TestAuthRejectsWithoutToken(t *testing.T) {
	srv := NewServer("/tmp", "/tmp", 0, "secret-token")
	req := httptest.NewRequest("GET", "/api/sessions", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthAcceptsBearerToken(t *testing.T) {
	srv := NewServer("/tmp", "/tmp", 0, "secret-token")
	req := httptest.NewRequest("GET", "/api/sessions", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code == 401 {
		t.Fatal("should not reject valid bearer token")
	}
}

func TestAuthAcceptsCookie(t *testing.T) {
	srv := NewServer("/tmp", "/tmp", 0, "secret-token")
	req := httptest.NewRequest("GET", "/api/sessions", nil)
	req.AddCookie(&http.Cookie{Name: "kitestream_token", Value: "secret-token"})
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code == 401 {
		t.Fatal("should not reject valid cookie")
	}
}

func TestAuthRejectsWrongToken(t *testing.T) {
	srv := NewServer("/tmp", "/tmp", 0, "secret-token")
	req := httptest.NewRequest("GET", "/api/sessions", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("expected 401 for wrong token, got %d", w.Code)
	}
}

func TestRootSetsAuthCookie(t *testing.T) {
	srv := NewServer("/tmp", "/tmp", 0, "my-token")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "kitestream_token" && c.Value == "my-token" {
			found = true
			if !c.HttpOnly {
				t.Error("cookie should be HttpOnly")
			}
		}
	}
	if !found {
		t.Error("auth cookie not set on root request")
	}
}
