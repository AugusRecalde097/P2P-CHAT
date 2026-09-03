package ui

import (
	"fmt"
	"strings"
)

// PeerInfo describes a peer that can be shown in the ASCII network view.
type PeerInfo struct {
	Nick      string
	Connected bool
	Known     bool
}

// RenderNetwork builds a simple ASCII network visualization for the CLI.
func RenderNetwork(localNick string, peers []PeerInfo) string {
	local := strings.TrimSpace(localNick)
	if local == "" {
		local = "Tú"
	}

	visible := peers
	overflow := 0
	if len(visible) > 4 {
		overflow = len(visible) - 4
		visible = visible[:4]
	}

	var body []string
	switch len(visible) {
	case 0:
		body = renderEmpty(local)
	case 1:
		body = renderSingle(local, visible[0])
	case 2:
		body = renderDouble(local, visible[0], visible[1])
	case 3:
		body = renderTriple(local, visible[0], visible[1], visible[2])
	default:
		body = renderMany(local, visible, overflow)
	}

	return renderBox(body)
}

func renderEmpty(local string) []string {
	return []string{
		"      ●",
		"     (" + local + ")",		"      solo",	}
}

func renderSingle(local string, peer PeerInfo) []string {
	return []string{
		"      " + label(peer),
		"       │",
		"       ●",
		"       │",
		"      " + local,
	}
}

func renderDouble(local string, first, second PeerInfo) []string {

	return []string{
		"      " + label(first),
		"       │",
		"       ● ── " + label(second),
		"       │",
		"      " + local,
	}

}

func renderTriple(local string, first, second, third PeerInfo) []string {
	return []string{
		"      " + label(first),
		"       │",
		fmt.Sprintf("%s ─── ● ─── %s", label(second), label(third)),
		"       │",
		"      " + local,
	}
}

func renderMany(local string, peers []PeerInfo, overflow int) []string {
	lines := []string{
		"       " + label(peers[0]),
		"        │",
		fmt.Sprintf("%s ───● ─── %s", label(peers[1]), label(peers[2])),
		"        │",
		"       " + label(peers[3]),
		"        │",
		"       " + local,
	}
	if overflow > 0 {
		lines = append(lines, fmt.Sprintf("      +%d más", overflow))
	}
	return lines
}

func renderBox(body []string) string {
	width := 30
	innerWidth := width - 2
	lines := make([]string, 0, len(body)+4)
	lines = append(lines, "╭"+strings.Repeat("─", innerWidth)+"╮")
	lines = append(lines, "         P2P Network         ")
	lines = append(lines, "                             ")
	for _, line := range body {
		content := fmt.Sprintf("%-"+fmt.Sprint(innerWidth)+"s", line)
		lines = append(lines, " "+content+" ")
	}
	lines = append(lines, "╰"+strings.Repeat("─", innerWidth)+"╯")
	return strings.Join(lines, "\n")
}

func truncate(value string, width int) string {
	if len(value) <= width {
		return value
	}
	if width <= 10 {
		return value[:width]
	}
	return value[:width-10] + "..."
}

func label(peer PeerInfo) string {
	name := strings.TrimSpace(peer.Nick)
	if name == "" {
		name = "desconocido"
	}
	return name
}

func formatPair(left, right string) string {
	left = truncate(left, 12)
	right = truncate(right, 12)
	return fmt.Sprintf("%-12s %-12s", left, right)
}
