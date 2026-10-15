// SPDX-License-Identifier: GPL-2.0-only
// SPDX-FileCopyrightText: 2026 Mono Technologies Inc.

package widget

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestDecodeOverviewBodyWhitelist(t *testing.T) {
	good := []string{
		`{"type":"stack","children":[{"type":"properties","items":[{"label":"Device","value":"ODU"}]},{"type":"callout","variant":"info","body":"Ready"}]}`,
		`{"type":"properties","items":[]}`,
		`{"type":"callout","variant":"warning","title":"Unavailable","body":"No data"}`,
	}
	for _, raw := range good {
		if _, err := DecodeOverviewBody(json.RawMessage(raw)); err != nil {
			t.Errorf("accepted overview body rejected: %s: %v", raw, err)
		}
	}
	bad := []string{
		`{"type":"form","style":"page"}`,
		`{"type":"stack","children":[{"type":"link","label":"go","href":"/outside"}]}`,
		`{"type":"callout","body":"go","link":{"type":"link","label":"go","href":"/plugins/x/"}}`,
		`{"type":"callout","body":"go","acts":[{"label":"run","href":"/plugins/x/"}]}`,
	}
	for _, raw := range bad {
		if _, err := DecodeOverviewBody([]byte(raw)); err == nil {
			t.Errorf("accepted forbidden overview body: %s", raw)
		}
	}
}

func TestOverviewLayoutPluginIDsAreKnownAndRoundTrip(t *testing.T) {
	id := PluginOverviewCardID("jodu-5174x")
	value := "1.internet-" + id + ".traffic-ports-tunnels-resources-system-hardware.."
	got := ParseOverviewLayoutWithPlugins(value, []string{"jodu-5174x"})
	if got.Encode() != value {
		t.Fatalf("round trip = %q, want %q", got.Encode(), value)
	}
	unknown := ParseOverviewLayoutWithPlugins("1.internet-"+id+".traffic-ports-tunnels-resources-system-hardware..", nil)
	if got := slices.Concat(unknown.Cols[0], unknown.Cols[1]); slices.Contains(got, id) {
		t.Fatalf("uninstalled plugin card survived layout: %v", got)
	}
}

func TestOverviewLayoutPreservesAllCardsHidden(t *testing.T) {
	ids := append(overviewCardIDs(), PluginOverviewCardID("jodu-5174x"))
	encoded := "1...." + strings.Join(ids, "-")
	l := ParseOverviewLayoutWithPlugins(encoded, []string{"jodu-5174x"})
	o := &Overview{Layout: l, PluginCards: []OverviewPluginCard{{PluginID: "jodu-5174x"}}}
	shown := o.layout()
	if len(shown.Cols[0])+len(shown.Cols[1]) != 0 || !slices.Equal(shown.Hidden, ids) {
		t.Fatalf("all-hidden layout became %+v", shown)
	}
}
