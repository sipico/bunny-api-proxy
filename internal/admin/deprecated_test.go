package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/sipico/bunny-api-proxy/internal/auth"
	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// Record type "SPF" was the proxy's name for bunny.net type 5, which the spec
// now calls "Redirect". Stored "SPF" permissions are kept but match nothing,
// new grants are refused, and the admin is warned (no silent conversion).

func spfPermission() *storage.Permission {
	return &storage.Permission{ID: 7, TokenID: 2, ZoneID: 123,
		AllowedActions: []string{"add_record"}, RecordTypes: []string{"TXT", "SPF"}}
}

func TestValidatePermissionValues_RejectsSPFWithExplanation(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	if validatePermissionValues(w, []string{"add_record"}, []string{"TXT", "SPF"}) {
		t.Fatal("SPF must be rejected")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"can no longer be granted", "Redirect", "TXT"} {
		if !strings.Contains(body, want) {
			t.Errorf("response %q does not mention %q", body, want)
		}
	}

	ok := httptest.NewRecorder()
	if !validatePermissionValues(ok, []string{"add_record"}, []string{"Redirect", "SVCB", "HTTPS", "TLSA"}) {
		t.Errorf("new spec record types must be accepted: %s", ok.Body.String())
	}
}

func TestHandleGetUnifiedToken_WarnsAboutSPF(t *testing.T) {
	t.Parallel()
	mock := newMockUnifiedStorage()
	mock.GetTokenByIDFunc = func(ctx context.Context, id int64) (*storage.Token, error) {
		return &storage.Token{ID: 2, Name: "acme", CreatedAt: time.Now()}, nil
	}
	mock.GetPermissionsForTokenFunc = func(ctx context.Context, tokenID int64) ([]*storage.Permission, error) {
		return []*storage.Permission{spfPermission()}, nil
	}
	h := NewHandler(mock, new(slog.LevelVar), slog.Default())

	req := httptest.NewRequest("GET", "/api/tokens/2", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "2")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.HandleGetUnifiedToken(w, req)

	var resp UnifiedTokenDetailResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Warnings) != 1 || !strings.Contains(resp.Warnings[0], "permission 7 (zone 123)") {
		t.Errorf("warnings = %q, want one for permission 7", resp.Warnings)
	}
	// The stored value is shown as it is, not rewritten.
	if got := resp.Permissions[0].RecordTypes; len(got) != 2 || got[1] != "SPF" {
		t.Errorf("record types = %v, want the stored [TXT SPF]", got)
	}
}

func TestHandleWhoami_WarnsAboutSPF(t *testing.T) {
	t.Parallel()
	mock := newMockUnifiedStorage()
	mock.GetPermissionsForTokenFunc = func(ctx context.Context, tokenID int64) ([]*storage.Permission, error) {
		return []*storage.Permission{spfPermission()}, nil
	}
	h := NewHandler(mock, new(slog.LevelVar), slog.Default())

	req := httptest.NewRequest("GET", "/api/whoami", nil)
	req = req.WithContext(auth.WithToken(req.Context(), &storage.Token{ID: 2, Name: "acme"}))
	w := httptest.NewRecorder()
	h.HandleWhoami(w, req)

	var resp WhoamiResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Warnings) != 1 {
		t.Errorf("warnings = %q, want 1", resp.Warnings)
	}
}

func TestLogDeprecatedPermissions(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	mock := newMockUnifiedStorage()
	mock.ListTokensFunc = func(ctx context.Context) ([]*storage.Token, error) {
		return []*storage.Token{
			{ID: 1, Name: "admin", IsAdmin: true},
			{ID: 2, Name: "acme"},
			{ID: 3, Name: "clean"},
			{ID: 4, Name: "broken"},
		}, nil
	}
	mock.GetPermissionsForTokenFunc = func(ctx context.Context, tokenID int64) ([]*storage.Permission, error) {
		switch tokenID {
		case 1:
			t.Error("admin tokens have no permissions and must not be queried")
		case 2:
			return []*storage.Permission{spfPermission()}, nil
		case 4:
			return nil, errors.New("disk error")
		}
		return []*storage.Permission{{ID: 9, ZoneID: 1, RecordTypes: []string{"TXT"}}}, nil
	}
	NewHandler(mock, new(slog.LevelVar), logger).LogDeprecatedPermissions(context.Background())

	out := buf.String()
	if strings.Count(out, "record type that no longer exists") != 1 {
		t.Errorf("want exactly one warning (token acme), got:\n%s", out)
	}
	for _, want := range []string{"token_name=acme", "permission_id=7", "zone_id=123", "token_id=4"} {
		if !strings.Contains(out, want) {
			t.Errorf("log does not contain %q:\n%s", want, out)
		}
	}
}

func TestLogDeprecatedPermissions_ListError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	mock := newMockUnifiedStorage()
	mock.ListTokensFunc = func(ctx context.Context) ([]*storage.Token, error) {
		return nil, errors.New("db locked")
	}
	NewHandler(mock, new(slog.LevelVar), slog.New(slog.NewTextHandler(&buf, nil))).LogDeprecatedPermissions(context.Background())
	if !strings.Contains(buf.String(), "db locked") {
		t.Errorf("list error not logged: %s", buf.String())
	}
}
