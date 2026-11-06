package protocol

import "encoding/json"

type Message struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	FromID    string          `json:"from_id"`
	FromNick  string          `json:"from_nick"`
	ToID      *string         `json:"to_id,omitempty"`
	Timestamp int64           `json:"timestamp"`
	TTL       int             `json:"ttl,omitempty"`
	Payload   json.RawMessage `json:"payload"`
	IV        string          `json:"iv,omitempty"`
	Encrypted bool            `json:"encrypted,omitempty"`
}

type ChatPayload struct {
	Message string `json:"message"`
}

type PeerInfo struct {
	NodeID string `json:"node_id"`
	Nick   string `json:"nick"`
}

type HandshakePayload struct {
	NodeID        string     `json:"node_id"`
	Nick          string     `json:"nick"`
	PublicKey     string     `json:"public_key"`
	KnownPeers    []PeerInfo `json:"known_peers,omitempty"`
	SignPublicKey string     `json:"sign_public_key"`
	Signature     string     `json:"signature,omitempty"`
}

type AckPayload struct {
	MessageID string `json:"message_id"`
}
