package peer

import (
	"crypto/ecdh"
	"encoding/json"
	"net"
	"sync"
	"time"

	"p2p-chat/protocol"
)

type MessageHandler interface {
	HandleMessage(p *Peer, msg protocol.Message)
	RemovePeer(id string)
}

type Peer struct {
	ID            string
	Nick          string
	Conn          net.Conn
	handler       MessageHandler
	Identified    bool
	SharedKey     []byte           // clave compartida AES
	PrivateKey    *ecdh.PrivateKey // clave privada ECDH
	SignPublicKey string
	// rate limiting
	RateLimit   int
	RateWindow  time.Duration
	msgCount    int
	windowStart time.Time
	mu          sync.Mutex
}


func NewPeer(conn net.Conn, h MessageHandler) *Peer {
    return &Peer{
        Conn:    conn,
        handler: h,
    }
}

func (p *Peer) ReadLoop() {
    decoder := json.NewDecoder(p.Conn)

    for {
        var msg protocol.Message
        if err := decoder.Decode(&msg); err != nil {
			p.Close()
			break
        }

		// apply simple rate limiting per peer (token-bucket like window)
		if p.RateWindow == 0 {
			p.RateWindow = time.Second
		}
		if p.RateLimit == 0 {
			p.RateLimit = 20
		}

		p.mu.Lock()
		now := time.Now()
		if p.windowStart.IsZero() || now.Sub(p.windowStart) > p.RateWindow {
			p.windowStart = now
			p.msgCount = 0
		}
		p.msgCount++
		tooMany := p.msgCount > p.RateLimit
		p.mu.Unlock()

		if tooMany {
			// rate limit exceeded
			p.Close()
			break
		}

		p.handler.HandleMessage(p, msg)
    }

    p.handler.RemovePeer(p.ID)
}

func (p *Peer) Send(msg protocol.Message) {
	encoder := json.NewEncoder(p.Conn)
	encoder.Encode(msg)
}

func (p *Peer) Close() {
	p.Conn.Close()
}
