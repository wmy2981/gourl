package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestUpdateConfigValidationCodes: PUT /config reports one stable rule code
// per validation failure, with interpolation params where applicable. The
// PUT body is the full config, so every case carries a valid baseline and
// breaks exactly one field.
func TestUpdateConfigValidationCodes(t *testing.T) {
	s, _ := newTestServer(t)
	baseline := func() map[string]any {
		return map[string]any{
			"site":              map[string]any{"name": "x"},
			"short_code_length": 6,
		}
	}

	t.Run("short_code_length_range", func(t *testing.T) {
		body := baseline()
		body["short_code_length"] = 1
		rec := do(t, s, http.MethodPut, "/api/v1/config", body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		var eb errorBody
		if err := json.Unmarshal(rec.Body.Bytes(), &eb); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if eb.Error.Code != "short_code_length_range" {
			t.Errorf("code = %q, want short_code_length_range", eb.Error.Code)
		}
		if eb.Error.Params["got"] != float64(1) || eb.Error.Params["min"] != float64(2) || eb.Error.Params["max"] != float64(64) {
			t.Errorf("params = %v, want got=1 min=2 max=64", eb.Error.Params)
		}
	})

	t.Run("invalid_log_level", func(t *testing.T) {
		body := baseline()
		body["log_level"] = "verbose"
		rec := do(t, s, http.MethodPut, "/api/v1/config", body)
		var eb errorBody
		if err := json.Unmarshal(rec.Body.Bytes(), &eb); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if eb.Error.Code != "invalid_log_level" {
			t.Errorf("code = %q, want invalid_log_level", eb.Error.Code)
		}
		if eb.Error.Params["got"] != "verbose" {
			t.Errorf("params = %v, want got=verbose", eb.Error.Params)
		}
	})

	t.Run("invalid_ip_block", func(t *testing.T) {
		body := baseline()
		body["ip_blocks"] = []string{"999.1.2.3"}
		rec := do(t, s, http.MethodPut, "/api/v1/config", body)
		var eb errorBody
		if err := json.Unmarshal(rec.Body.Bytes(), &eb); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if eb.Error.Code != "invalid_ip_block" {
			t.Errorf("code = %q, want invalid_ip_block", eb.Error.Code)
		}
		if eb.Error.Params["entry"] != "999.1.2.3" {
			t.Errorf("params = %v, want entry=999.1.2.3", eb.Error.Params)
		}
	})
}
