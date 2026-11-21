package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const moveBody = `{"game":{"id":"game","ruleset":{"name":"standard"},"timeout":500},"board":{"width":11,"height":11,"snakes":[{"id":"ours"},{"id":"theirs"}]},"you":{"id":"ours","health":90,"head":{"x":1,"y":1},"body":[{"x":1,"y":1}]}}`

func TestHandler(t *testing.T) {
	handler := newHandler(func(mode Mode, request MoveRequest) string {
		if mode != Standard || request.You.ID != "ours" {
			t.Fatalf("got mode %q and snake %q", mode, request.You.ID)
		}
		return "right"
	})

	tests := []struct {
		method, path, body string
		status             int
		field, value       string
	}{
		{http.MethodGet, "/", "", 200, "apiversion", "1"},
		{http.MethodGet, "/royale", "", 200, "apiversion", "1"},
		{http.MethodPost, "/constrictor/start", "", 200, "", ""},
		{http.MethodPost, "/end", "", 200, "", ""},
		{http.MethodPost, "/standard/move", moveBody, 200, "move", "right"},
		{http.MethodPost, "/move", "{", 400, "", ""},
		{http.MethodPost, "/move", "null", 400, "", ""},
		{http.MethodGet, "/move", "", 405, "", ""},
		{http.MethodPost, "/missing/move", moveBody, 404, "", ""},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%s %s %d", test.method, test.path, test.status), func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, strings.NewReader(test.body)))
			if response.Code != test.status {
				t.Fatalf("status %d, want %d; body: %s", response.Code, test.status, response.Body)
			}
			if test.field != "" {
				var body map[string]string
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body[test.field] != test.value {
					t.Fatalf("response %s, want %s=%s", response.Body, test.field, test.value)
				}
			}
		})
	}
}
