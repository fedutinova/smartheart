package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/fedutinova/smartheart/back-api/apperr"
)

// ---------------------------------------------------------------------------
// RequestReset
// ---------------------------------------------------------------------------

func TestRequestReset_MissingEmail(t *testing.T) {
	d := newTestDeps(t)
	h := d.handler()

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest("POST", "/v1/auth/password-reset", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Password.RequestReset(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRequestReset_InvalidEmail(t *testing.T) {
	d := newTestDeps(t)
	h := d.handler()

	body, _ := json.Marshal(map[string]string{"email": "not-an-email"})
	req := httptest.NewRequest("POST", "/v1/auth/password-reset", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Password.RequestReset(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRequestReset_Success(t *testing.T) {
	d := newTestDeps(t)

	d.passwordSvc.EXPECT().
		RequestReset(mock.Anything, "alice@example.com").
		Return(nil)

	h := d.handler()

	body, _ := json.Marshal(map[string]string{"email": "alice@example.com"})
	req := httptest.NewRequest("POST", "/v1/auth/password-reset", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Password.RequestReset(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// ConfirmReset
// ---------------------------------------------------------------------------

func TestConfirmReset_MissingFields(t *testing.T) {
	d := newTestDeps(t)
	h := d.handler()

	body, _ := json.Marshal(map[string]string{"token": "abc"})
	req := httptest.NewRequest("POST", "/v1/auth/password-reset/confirm", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Password.ConfirmReset(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestConfirmReset_InvalidToken(t *testing.T) {
	d := newTestDeps(t)

	d.passwordSvc.EXPECT().
		ConfirmReset(mock.Anything, "badtoken", "strongpassword123").
		Return(apperr.ErrInvalidToken)

	h := d.handler()

	body, _ := json.Marshal(map[string]string{
		"token":        "badtoken",
		"new_password": "strongpassword123",
	})
	req := httptest.NewRequest("POST", "/v1/auth/password-reset/confirm", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Password.ConfirmReset(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestConfirmReset_Success(t *testing.T) {
	d := newTestDeps(t)

	d.passwordSvc.EXPECT().
		ConfirmReset(mock.Anything, "validtoken", "strongpassword123").
		Return(nil)

	h := d.handler()

	body, _ := json.Marshal(map[string]string{
		"token":        "validtoken",
		"new_password": "strongpassword123",
	})
	req := httptest.NewRequest("POST", "/v1/auth/password-reset/confirm", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Password.ConfirmReset(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// ChangePassword
// ---------------------------------------------------------------------------

func TestChangePassword_Unauthenticated(t *testing.T) {
	d := newTestDeps(t)
	h := d.handler()

	body, _ := json.Marshal(map[string]string{
		"old_password": "oldpassword123",
		"new_password": "newpassword123",
	})
	req := httptest.NewRequest("POST", "/v1/auth/password-change", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Password.ChangePassword(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestChangePassword_WrongOldPassword(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.New()

	d.passwordSvc.EXPECT().
		ChangePassword(mock.Anything, userID, "wrongpassword1", "newstrongpass123").
		Return(apperr.ErrInvalidCredentials)

	h := d.handler()

	body, _ := json.Marshal(map[string]string{
		"old_password": "wrongpassword1",
		"new_password": "newstrongpass123",
	})
	req := httptest.NewRequest("POST", "/v1/auth/password-change", bytes.NewReader(body))
	req = withAuthContext(req, userID, []string{"user"})
	w := httptest.NewRecorder()

	h.Password.ChangePassword(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestChangePassword_Success(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.New()

	d.passwordSvc.EXPECT().
		ChangePassword(mock.Anything, userID, "oldpassword123", "newstrongpass123").
		Return(nil)

	h := d.handler()

	body, _ := json.Marshal(map[string]string{
		"old_password": "oldpassword123",
		"new_password": "newstrongpass123",
	})
	req := httptest.NewRequest("POST", "/v1/auth/password-change", bytes.NewReader(body))
	req = withAuthContext(req, userID, []string{"user"})
	w := httptest.NewRecorder()

	h.Password.ChangePassword(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
