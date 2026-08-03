package handler

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/strelov1/freehire/internal/auth"
	"github.com/strelov1/freehire/internal/discordbot"
)

// discordApp mounts the interaction webhook on an enabled handler signing with
// the given Ed25519 key pair. No DB is needed: the signature guard, PING, and
// an invalid-token /link reply never reach the queries field.
func discordApp(pub ed25519.PublicKey) (*fiber.App, *discordHandlers) {
	h := &discordHandlers{
		discordBot:       discordbot.NewClient("bottoken"),
		discordLinks:     discordbot.NewDiscordLinkTokens("test-secret", 10*time.Minute),
		discordPublicKey: hex.EncodeToString(pub),
	}
	app := fiber.New(fiber.Config{ErrorHandler: RenderError})
	app.Post("/api/v1/discord/interactions", h.DiscordInteraction)
	return app, h
}

// signedRequest builds an interaction POST with a valid Ed25519 signature over
// timestamp||body, as Discord computes it.
func signedRequest(t *testing.T, priv ed25519.PrivateKey, timestamp string, body []byte) *http.Request {
	t.Helper()
	sig := ed25519.Sign(priv, append([]byte(timestamp), body...))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discord/interactions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature-Ed25519", hex.EncodeToString(sig))
	req.Header.Set("X-Signature-Timestamp", timestamp)
	return req
}

func TestDiscordInteraction_missingSignatureForbidden(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	app, _ := discordApp(pub)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/discord/interactions", bytes.NewReader([]byte(`{"type":1}`)))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", res.StatusCode)
	}
}

func TestDiscordInteraction_invalidSignatureForbidden(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	// Sign with a DIFFERENT key than the one the handler verifies against.
	_, otherPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	app, _ := discordApp(pub)

	body := []byte(`{"type":1}`)
	req := signedRequest(t, otherPriv, "1700000000", body)
	res, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", res.StatusCode)
	}
}

// TestDiscordInteraction_tamperedBodyForbidden guards that the signature covers
// the RAW body actually processed: a body that would parse fine but does not
// match the signed bytes must still be rejected before any JSON parsing.
func TestDiscordInteraction_tamperedBodyForbidden(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	app, _ := discordApp(pub)

	signedBody := []byte(`{"type":1}`)
	sig := ed25519.Sign(priv, append([]byte("1700000000"), signedBody...))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discord/interactions", bytes.NewReader([]byte(`{"type":2}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature-Ed25519", hex.EncodeToString(sig))
	req.Header.Set("X-Signature-Timestamp", "1700000000")
	res, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", res.StatusCode)
	}
}

func TestDiscordInteraction_ping(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	app, _ := discordApp(pub)

	req := signedRequest(t, priv, "1700000000", []byte(`{"type":1}`))
	res, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["type"] != float64(1) {
		t.Errorf("body = %+v, want type=1 (PONG)", out)
	}
}

// TestDiscordInteraction_linkCommand_invalidToken exercises command routing
// down to the /link handler with a garbage token. Parse fails before any DB
// access, so this is safe to run against a handler with nil queries — a panic
// here would mean the code tried to touch the DB despite the invalid token.
func TestDiscordInteraction_linkCommand_invalidToken(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	app, _ := discordApp(pub)

	body, err := json.Marshal(discordbot.Interaction{
		Type: discordbot.InteractionTypeApplicationCommand,
		Data: &discordbot.InteractionData{
			Name:    "link",
			Options: []discordbot.InteractionOption{{Name: "token", Value: "garbage-token"}},
		},
		Member: &discordbot.Member{User: &discordbot.User{ID: "123456789"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := signedRequest(t, priv, "1700000000", body)
	res, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (interaction responses are always 200)", res.StatusCode)
	}
	var out discordbot.Response
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Type != discordbot.ResponseTypeChannelMessageWithSource {
		t.Errorf("response type = %d, want %d (channel message)", out.Type, discordbot.ResponseTypeChannelMessageWithSource)
	}
	if out.Data == nil || out.Data.Flags != discordbot.FlagEphemeral {
		t.Errorf("response data = %+v, want ephemeral flag set", out.Data)
	}
}

// TestDiscordInteraction_unknownCommand guards that an unrecognized command
// name (e.g. "contribute", before a later task wires it up) replies with a
// generic ephemeral error instead of panicking on an unhandled shape.
func TestDiscordInteraction_unknownCommand(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	app, _ := discordApp(pub)

	body, err := json.Marshal(discordbot.Interaction{
		Type: discordbot.InteractionTypeApplicationCommand,
		Data: &discordbot.InteractionData{Name: "contribute"},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := signedRequest(t, priv, "1700000000", body)
	res, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var out discordbot.Response
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Data == nil || out.Data.Flags != discordbot.FlagEphemeral {
		t.Errorf("response data = %+v, want ephemeral flag set", out.Data)
	}
}

// TestNewDiscordHandlers_allOrNothingGate checks that the constructor's enable
// condition is genuinely all-or-nothing: any one of the four Discord config
// values missing must leave the feature disabled, not partially wired.
func TestNewDiscordHandlers_allOrNothingGate(t *testing.T) {
	full := []string{"token", "app-id", "pub-key", "guild-id"}
	tests := []struct {
		name                             string
		botToken, appID, pubKey, guildID string
	}{
		{"fully configured", full[0], full[1], full[2], full[3]},
		{"missing bot token", "", full[1], full[2], full[3]},
		{"missing application id", full[0], "", full[2], full[3]},
		{"missing public key", full[0], full[1], "", full[3]},
		{"missing guild id", full[0], full[1], full[2], ""},
		{"nothing configured", "", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newDiscordHandlers(nil, "jwt-secret", tt.botToken, tt.appID, tt.pubKey, tt.guildID, "", nil)
			want := tt.name == "fully configured"
			if got := h.discordEnabled(); got != want {
				t.Errorf("discordEnabled() = %v, want %v", got, want)
			}
		})
	}
}

func TestDiscordInteraction_disabledReturns404(t *testing.T) {
	h := &discordHandlers{} // no config → disabled
	app := fiber.New(fiber.Config{ErrorHandler: RenderError})
	app.Post("/api/v1/discord/interactions", h.DiscordInteraction)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/discord/interactions", bytes.NewReader([]byte(`{"type":1}`)))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", res.StatusCode)
	}
}

// TestDiscordLink_disabledReturns503 mirrors TestTelegramDisabledWhenUnconfigured:
// a partially- or un-configured bot must not mint a link token.
func TestDiscordLink_disabledReturns503(t *testing.T) {
	h := &discordHandlers{} // no config → disabled
	iss := auth.NewIssuer("test-secret", time.Hour)
	cookie, err := iss.Issue(1, testTokenVersion)
	if err != nil {
		t.Fatal(err)
	}

	app := fiber.New(fiber.Config{ErrorHandler: RenderError})
	app.Post("/api/v1/me/discord/link", auth.RequireAuth(iss, testVersions), h.LinkDiscord)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/discord/link", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	res, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", res.StatusCode)
	}
}
