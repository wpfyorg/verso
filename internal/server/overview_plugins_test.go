// SPDX-License-Identifier: GPL-2.0-only
// SPDX-FileCopyrightText: 2026 Mono Technologies Inc.

package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/we-are-mono/verso/internal/plugin"
	"github.com/we-are-mono/verso/internal/widget"
)

func overviewManifest() plugin.Manifest {
	return plugin.Manifest{ID: "jodu-5174x", Name: "5G <ODU>", Socket: "/run/jodu.sock", SchemaVersion: 1,
		Overview: &plugin.OverviewEntry{Path: "/overview", Icon: "radio"},
		ACL:      plugin.ACL{Read: []plugin.ACLScope{{Scope: "uci", Object: "jodu5174x", Function: "read"}}}}
}

func TestHomepageComposesPluginOverviewWithScopedSnapshot(t *testing.T) {
	var writes []uciWrite
	var readSIDs []string
	transport := &fakeTransport{env: &plugin.Envelope{SchemaVersion: 1, Status: http.StatusOK, Title: "ignored",
		Widget: json.RawMessage(`{"type":"properties","items":[{"label":"Device <tag>","value":"<img src=x onerror=1>"}]}`)}}
	backend := fakeBackend{writes: &writes, uciReadSIDs: &readSIDs, uci: map[string]map[string]any{"jodu5174x": {
		"main": map[string]any{".type": "jodu5174x", "host": "private-value"},
	}}}
	s := newServerWith(t, backend, transport, []plugin.Manifest{overviewManifest()})
	rec := get(t, s, "/")
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("homepage status %d", rec.Code)
	}
	id := widget.PluginOverviewCardID("jodu-5174x")
	for _, want := range []string{`data-overview-card="` + id + `"`, `>5G &lt;ODU&gt;</h2>`, `href="/plugins/jodu-5174x/"`, `Device &lt;tag&gt;`, `&lt;img src=x onerror=1&gt;`} {
		if !strings.Contains(body, want) {
			t.Errorf("homepage missing escaped plugin content %q", want)
		}
	}
	if strings.Contains(body, "private-value") || strings.Contains(body, "<img src=x") {
		t.Fatal("plugin data or markup escaped the widget renderer")
	}
	if transport.fetches != 1 || transport.lastSocket != "/run/jodu.sock" || transport.lastReq.Method != http.MethodGet || transport.lastReq.Path != "/overview" || transport.lastReq.Form != nil {
		t.Fatalf("plugin request = fetches %d socket %q request %+v", transport.fetches, transport.lastSocket, transport.lastReq)
	}
	if transport.lastReq.UCI["jodu5174x"]["main"].(map[string]any)["host"] != "private-value" || len(writes) != 0 {
		t.Fatalf("scoped read snapshot missing or write occurred: UCI=%v writes=%v", transport.lastReq.UCI, writes)
	}
	if !slices.Contains(readSIDs, "test-sid") {
		t.Fatalf("plugin snapshot was not read with the operator session: %v", readSIDs)
	}
}

func TestHomepageHiddenPluginCardDoesNotFetch(t *testing.T) {
	transport := &fakeTransport{env: &plugin.Envelope{SchemaVersion: 1, Status: http.StatusOK, Widget: json.RawMessage(`{"type":"properties","items":[]}`)}}
	s := newServerWith(t, fakeBackend{}, transport, []plugin.Manifest{overviewManifest()})
	id := widget.PluginOverviewCardID("jodu-5174x")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: overviewLayoutCookie, Value: "1....internet-ports-tunnels-traffic-resources-system-hardware-" + id})
	rec := serve(t, s, req)
	if rec.Code != http.StatusOK || transport.fetches != 0 {
		t.Fatalf("hidden card homepage status=%d, plugin fetches=%d", rec.Code, transport.fetches)
	}
	if !strings.Contains(rec.Body.String(), `data-overview-restore="`+id+`"`) {
		t.Fatal("hidden plugin card missing from the Arrange shelf")
	}
	if strings.Contains(rec.Body.String(), `data-overview-card="`+id+`"`) {
		t.Fatal("all-hidden layout put plugin card back onto the Overview")
	}
}

func TestHomepageContainsFailedAndUnsafePluginCards(t *testing.T) {
	for _, tc := range []struct {
		name string
		tr   *fakeTransport
	}{
		{name: "transport failure", tr: &fakeTransport{err: errors.New("offline")}},
		{name: "write envelope", tr: &fakeTransport{env: &plugin.Envelope{SchemaVersion: 1, Status: http.StatusOK, Commit: []plugin.CommitOp{{Config: "jodu5174x"}}, Widget: json.RawMessage(`{"type":"properties","items":[]}`)}}},
		{name: "command envelope", tr: &fakeTransport{env: &plugin.Envelope{SchemaVersion: 1, Status: http.StatusOK, Commands: []plugin.ApplyAction{{Name: "restart"}}, Widget: json.RawMessage(`{"type":"properties","items":[]}`)}}},
		{name: "act envelope", tr: &fakeTransport{env: &plugin.Envelope{SchemaVersion: 1, Status: http.StatusOK, Act: json.RawMessage(`{"label":"run","href":"/plugins/jodu-5174x/run"}`), Widget: json.RawMessage(`{"type":"properties","items":[]}`)}}},
		{name: "immediate envelope", tr: &fakeTransport{env: &plugin.Envelope{SchemaVersion: 1, Status: http.StatusOK, Immediate: true, Widget: json.RawMessage(`{"type":"properties","items":[]}`)}}},
		{name: "form body", tr: &fakeTransport{env: &plugin.Envelope{SchemaVersion: 1, Status: http.StatusOK, Widget: json.RawMessage(`{"type":"form","style":"page"}`)}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var writes []uciWrite
			s := newServerWith(t, fakeBackend{writes: &writes}, tc.tr, []plugin.Manifest{overviewManifest()})
			rec := get(t, s, "/")
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "This plugin’s Overview reading is unavailable.") {
				t.Fatalf("failure escaped homepage: status=%d body contains unavailable=%v", rec.Code, strings.Contains(rec.Body.String(), "unavailable"))
			}
			if len(writes) != 0 {
				t.Fatalf("overview attempted config writes: %v", writes)
			}
		})
	}
}

func TestHomepageUnavailableWhenOperatorSnapshotMissing(t *testing.T) {
	transport := &fakeTransport{env: &plugin.Envelope{SchemaVersion: 1, Status: http.StatusOK, Widget: json.RawMessage(`{"type":"callout","variant":"warning","title":"Unavailable","body":"No authorized configuration"}`)}}
	s := newServerWith(t, fakeBackend{uciReadErr: errors.New("denied")}, transport, []plugin.Manifest{overviewManifest()})
	rec := get(t, s, "/")
	if rec.Code != http.StatusOK || transport.lastReq.UCI["jodu5174x"] != nil || !strings.Contains(rec.Body.String(), "No authorized configuration") {
		t.Fatalf("missing/denied read was not contained: status=%d UCI=%v", rec.Code, transport.lastReq.UCI)
	}
}
