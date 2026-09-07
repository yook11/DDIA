package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ddia/app/user"
)

type userRepositoryFunc func(context.Context, string) (user.User, error)

func (f userRepositoryFunc) CreateUser(ctx context.Context, name string) (user.User, error) {
	return f(ctx, name)
}

func TestCreateUserReturnsCreatedJSON(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"アリス"}`))
	called := false
	handler := NewUserHandler(user.NewService(userRepositoryFunc(func(ctx context.Context, name string) (user.User, error) {
		called = true
		if ctx != request.Context() || name != "アリス" {
			t.Fatalf("request changed: ctx=%v name=%q", ctx, name)
		}
		return user.User{ID: "42", Name: name}, nil
	})))
	response := httptest.NewRecorder()
	handler.Create(response, request)
	if !called || response.Code != http.StatusCreated {
		t.Fatalf("status=%d called=%v body=%s", response.Code, called, response.Body)
	}
	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q", response.Header().Get("Content-Type"))
	}
	var got map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["id"] != "42" || got["name"] != "アリス" {
		t.Fatalf("JSON = %v", got)
	}
}

func TestCreateUserRejectsInvalidJSONWithoutCallingService(t *testing.T) {
	tests := map[string]string{
		"empty":       "",
		"syntax":      `{"name":`,
		"wrong type":  `{"name":42}`,
		"missing":     `{}`,
		"null name":   `{"name":null}`,
		"null object": `null`,
		"array":       `[]`,
		"trailing":    `{"name":"alice"} {}`,
		"garbage":     `{"name":"alice"}garbage`,
		"too large":   `{"name":"` + strings.Repeat("a", 1<<20) + `"}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			handler := NewUserHandler(user.NewService(userRepositoryFunc(func(context.Context, string) (user.User, error) {
				t.Fatal("invalid JSON reached the repository")
				return user.User{}, nil
			})))
			response := httptest.NewRecorder()
			handler.Create(response, httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(body)))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
		})
	}
}

func TestCreateUserMapsErrorsWithoutExposingDetails(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"internal", errors.New("password=private-detail"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewUserHandler(user.NewService(userRepositoryFunc(func(context.Context, string) (user.User, error) {
				return user.User{}, tc.err
			})))
			response := httptest.NewRecorder()
			handler.Create(response, httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"alice"}`)))
			if response.Code != tc.status || strings.Contains(response.Body.String(), "private") {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
			var body map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || len(body) != 1 || body["error"] == "" {
				t.Fatalf("invalid error JSON: %s (%v)", response.Body, err)
			}
		})
	}
}
