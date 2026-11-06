package ui

import (
	"strings"
	"testing"
)

func TestRenderNetworkWithoutPeers(t *testing.T) {
	got := RenderNetwork("Tú", nil)

	if !strings.Contains(got, "Tú") {
		t.Fatalf("expected local nick in output, got %q", got)
	}
	if !strings.Contains(got, "solo") {
		t.Fatalf("expected solo state in output, got %q", got)
	}
}

func TestRenderNetworkShowsOverflowForManyPeers(t *testing.T) {
	peers := []PeerInfo{
		{Nick: "Bob", Connected: true},
		{Nick: "Ana", Connected: true},
		{Nick: "Carlos", Connected: true},
		{Nick: "Daniel", Connected: true},
		{Nick: "Eva", Connected: true},
	}

	got := RenderNetwork("Tú", peers)

	if !strings.Contains(got, "+1 más") {
		t.Fatalf("expected overflow hint in output, got %q", got)
	}
}
