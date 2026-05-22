package node

import (
	"container/list"
	"crypto/ecdh"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"p2p-chat/crypto"
	"p2p-chat/peer"
	"p2p-chat/protocol"

	"github.com/google/uuid"
)

type Event struct {
	Type string
	Data any
}

type cacheEntry struct {
	key string
}

type LRUCache struct {
	capacity int
	ll       *list.List
	items    map[string]*list.Element
}

func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		ll:       list.New(),
		items:    make(map[string]*list.Element),
	}
}

func (c *LRUCache) Add(key string) {
	if elem, ok := c.items[key]; ok {
		c.ll.MoveToFront(elem)
		return
	}

	if c.ll.Len() >= c.capacity {
		tail := c.ll.Back()
		if tail != nil {
			entry := tail.Value.(cacheEntry)
			delete(c.items, entry.key)
			c.ll.Remove(tail)
		}
	}

	elem := c.ll.PushFront(cacheEntry{key: key})
	c.items[key] = elem
}

func (c *LRUCache) Contains(key string) bool {
	if elem, ok := c.items[key]; ok {
		c.ll.MoveToFront(elem)
		return true
	}
	return false
}

type Node struct {
	ID             string
	Nick           string
	Port           string
	PrivateKey     *ecdh.PrivateKey
	PublicKey      string
	SignPrivateKey ed25519.PrivateKey
	SignPublicKey  string

	peers map[string]*peer.Peer
	mu    sync.RWMutex

	seenMessages    *LRUCache
	pendingMessages map[string]bool
	routes          map[string]string
	knownPeers      map[string]string
	history         []protocol.Message
	Events          chan Event
	// limits and rate-limiting
	MaxPeers        int
	MessageRateLimit int
	RateWindow      time.Duration
	KnownPeersLimit int
	RoutesLimit     int
}

func shortID(id string) string {
	if len(id) <= 6 {
		return id
	}
	return id[:6]
}

func NewNode(id, nick string) *Node {
	privKey, pubKey, _ := crypto.GenerateKeyPair()
	signPub, signPriv, _ := crypto.GenerateSigningKeyPair()
	signPubB64 := base64.StdEncoding.EncodeToString(signPub)
	return &Node{
		ID:             id,
		Nick:           nick,
		PrivateKey:     privKey,
		PublicKey:      pubKey,
		SignPrivateKey: signPriv,
		SignPublicKey:  signPubB64,
		peers:          make(map[string]*peer.Peer),
		Events:         make(chan Event, 20),
		seenMessages:   NewLRUCache(1024),
		pendingMessages: make(map[string]bool),
		routes:         make(map[string]string),
		knownPeers:     make(map[string]string),
		history:        make([]protocol.Message, 0),
		MaxPeers:        50,
		MessageRateLimit: 20,
		RateWindow:      time.Second,
		KnownPeersLimit: 100,
		RoutesLimit:     500,
	}
}

func (n *Node) StartListening(port string) {
	n.Port = port
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				fmt.Printf("Error accepting connection: %v\n", err)
				continue
			}
			// enforce max peers
			n.mu.RLock()
			curPeers := len(n.peers)
			n.mu.RUnlock()
			if curPeers >= n.MaxPeers {
				fmt.Printf("connection refused: max peers reached (%d)\n", n.MaxPeers)
				conn.Close()
				continue
			}
			go n.handleNewConnection(conn)
		}
	}()
}

func (n *Node) handleNewConnection(conn net.Conn) {
	if conn == nil {
		return
	}

	p := peer.NewPeer(conn, n)

	// assign rate limiting to peer from node defaults
	p.RateLimit = n.MessageRateLimit
	p.RateWindow = n.RateWindow

	go p.ReadLoop()

	// enviar handshake también en conexiones entrantes
	p.Send(n.CreateHandshake())
}

func (n *Node) Connect(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}

	p := peer.NewPeer(conn, n)
	p.RateLimit = n.MessageRateLimit
	p.RateWindow = n.RateWindow
	go p.ReadLoop()

	p.Send(n.CreateHandshake())

	return nil
}

func (n *Node) AddPeer(p *peer.Peer) {
	n.mu.Lock()
	if _, exists := n.peers[p.ID]; exists {
		n.mu.Unlock()
		p.Close()
		return
	}

	n.peers[p.ID] = p
	n.routes[p.ID] = p.ID
	if p.Nick != "" {
		n.knownPeers[p.ID] = p.Nick
	}
	n.mu.Unlock()

	// Informar al resto de peers sobre este nuevo vecino para mejorar el enrutamiento.
	n.Broadcast(n.CreateHandshake(), p.ID)
	n.Events <- Event{Type: "system", Data: p.Nick + " conectado"}
}

func (n *Node) RemovePeer(id string) {
	n.mu.Lock()
	p, ok := n.peers[id]
	if ok {
		delete(n.peers, id)
		for dest, via := range n.routes {
			if via == id {
				delete(n.routes, dest)
			}
		}
	}
	n.mu.Unlock()

	if ok && p != nil && p.Nick != "" {
		n.Events <- Event{Type: "system", Data: p.Nick + " desconectado"}
	}
}

func (n *Node) Broadcast(msg protocol.Message, excludeID string) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for id, p := range n.peers {
		if id == excludeID {
			continue
		}
		go p.Send(msg)
	}
}

func (n *Node) HandleMessage(p *peer.Peer, msg protocol.Message) {
	switch msg.Type {

	case "handshake":
		var payload protocol.HandshakePayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			fmt.Printf("invalid handshake payload from %v: %v\n", p.Conn.RemoteAddr(), err)
			p.Close()
			return
		}

		if payload.NodeID == "" || payload.PublicKey == "" {
			fmt.Printf("invalid handshake fields from %v: missing NodeID or PublicKey\n", p.Conn.RemoteAddr())
			p.Close()
			return
		}

		if payload.SignPublicKey == "" || payload.Signature == "" {
			fmt.Printf("unsigned handshake from %v\n", p.Conn.RemoteAddr())
			p.Close()
			return
		}

		signaturePayload := protocol.HandshakePayload{
			NodeID:        payload.NodeID,
			Nick:          payload.Nick,
			PublicKey:     payload.PublicKey,
			KnownPeers:    payload.KnownPeers,
			SignPublicKey: payload.SignPublicKey,
		}
		signedData, err := json.Marshal(signaturePayload)
		if err != nil {
			fmt.Printf("failed to marshal handshake for verification from %v: %v\n", p.Conn.RemoteAddr(), err)
			p.Close()
			return
		}
		valid, err := crypto.VerifySignature(payload.SignPublicKey, signedData, payload.Signature)
		if err != nil || !valid {
			fmt.Printf("invalid handshake signature from %v: %v\n", p.Conn.RemoteAddr(), err)
			p.Close()
			return
		}

		if p.Identified {
			if payload.NodeID == p.ID && payload.Nick != p.Nick {
				p.Nick = payload.Nick
				n.Events <- Event{Type: "system", Data: payload.Nick + " actualizó su nick"}
			}
			return
		}

		if payload.NodeID == n.ID {
			fmt.Printf("duplicate node ID in handshake from %v\n", p.Conn.RemoteAddr())
			p.Close()
			return
		}

		p.ID = payload.NodeID
		p.Nick = payload.Nick
		p.SignPublicKey = payload.SignPublicKey
		p.Identified = true

		// Derivar clave compartida desde la clave pública recibida
		sharedKey, err := crypto.DeriveSharedSecret(n.PrivateKey, payload.PublicKey)
		if err == nil {
			p.SharedKey = sharedKey
		} else {
			fmt.Printf("failed to derive shared secret from %v: %v\n", p.ID, err)
		}

		n.AddPeer(p)
		n.mu.Lock()
		n.knownPeers[p.ID] = p.Nick
		n.mu.Unlock()
		n.updateRoutesFromHandshake(p, payload)

		p.Send(n.CreateHandshake())

	case "chat":
		if n.seenMessages.Contains(msg.ID) {
			return
		}

		if msg.TTL <= 0 {
			return
		}

		if msg.Encrypted && len(msg.IV) > 0 {
			var ciphertext string
			if err := json.Unmarshal(msg.Payload, &ciphertext); err != nil {
				fmt.Printf("invalid encrypted chat payload from %v: %v\n", p.ID, err)
				return
			}
			plaintext, err := crypto.DecryptPayload(ciphertext, msg.IV, p.SharedKey)
			if err != nil {
				fmt.Printf("failed to decrypt chat from %v: %v\n", p.ID, err)
				return
			}
			msg.Payload = plaintext
			msg.Encrypted = false
			msg.IV = ""
		}

		n.seenMessages.Add(msg.ID)

		if msg.ToID != nil && *msg.ToID != n.ID {
			n.forwardMessage(msg, p.ID)
			return
		}

		if msg.ToID == nil {
			n.RecordHistory(msg)
			n.Events <- Event{Type: "chat", Data: msg}
			msg.TTL--
			n.Broadcast(msg, p.ID) // excluir origen
			return
		}

		n.RecordHistory(msg)
		ack := n.CreateAck(msg.ID, msg.FromID)
		n.SendTo(msg.FromID, ack)
		n.Events <- Event{Type: "chat", Data: msg}

	case "ack":
		if msg.ToID != nil && *msg.ToID != n.ID {
			n.forwardMessage(msg, p.ID)
			return
		}

		var payload protocol.AckPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			fmt.Printf("invalid ack payload from %v: %v\n", p.ID, err)
			return
		}

		n.mu.Lock()
		delete(n.pendingMessages, payload.MessageID)
		n.mu.Unlock()

		n.Events <- Event{Type: "ack", Data: payload.MessageID}
	}
}

func (n *Node) SendTo(id string, msg protocol.Message) {
	routeID, ok := n.routeFor(id)
	if !ok {
		return
	}

	if msg.TTL == 0 {
		msg.TTL = 8
	}

	n.mu.RLock()
	p, ok := n.peers[routeID]
	n.mu.RUnlock()
	if !ok {
		return
	}

	n.sendMessageToPeer(p, msg)

	n.mu.Lock()
	n.pendingMessages[msg.ID] = true
	n.mu.Unlock()
}

func (n *Node) sendMessageToPeer(p *peer.Peer, msg protocol.Message) {
	if p.SharedKey != nil && msg.Type == "chat" {
		ciphertext, iv, err := crypto.EncryptPayload(msg.Payload, p.SharedKey)
		if err == nil {
			msg.Payload = []byte("\"" + ciphertext + "\"")
			msg.IV = iv
			msg.Encrypted = true
		}
	}
	go p.Send(msg)
}

func (n *Node) routeFor(destID string) (string, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if _, ok := n.peers[destID]; ok {
		return destID, true
	}

	nextHop, ok := n.routes[destID]
	return nextHop, ok
}

func (n *Node) forwardMessage(msg protocol.Message, excludeID string) {
	if msg.TTL <= 0 {
		return
	}

	if msg.ToID != nil {
		destPeerID, ok := n.routeFor(*msg.ToID)
		if ok && destPeerID != excludeID {
			msg.TTL--
			n.SendTo(*msg.ToID, msg)
			return
		}
	}

	n.mu.RLock()
	for id, p := range n.peers {
		if id == excludeID {
			continue
		}
		msg.TTL--
		n.sendMessageToPeer(p, msg)
	}
	n.mu.RUnlock()
}

func (n *Node) ListPeers() []string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	list := []string{}
	for id, p := range n.peers {
		encrypted := ""
		status := "sin handshake"
		if p.SharedKey != nil {
			encrypted = " 🔒"
			status = "encriptado"
		} else if p.Identified {
			status = "esperando clave"
		}
		list = append(list, shortID(id)+" ("+p.Nick+")"+encrypted+" — "+status)
	}

	for id, via := range n.routes {
		if id == n.ID {
			continue
		}
		if _, direct := n.peers[id]; direct {
			continue
		}
		nick := n.knownPeers[id]
		if nick == "" {
			nick = "desconocido"
		}
		list = append(list, shortID(id)+" ("+nick+") — indirect vía "+shortID(via))
	}

	return list
}

func (n *Node) ResolveShortID(short string) (string, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for id := range n.peers {
		if strings.HasPrefix(id, short) {
			return id, true
		}
	}

	for id := range n.routes {
		if strings.HasPrefix(id, short) {
			return id, true
		}
	}
	return "", false
}

func (n *Node) UpdateNick(newNick string) {
	n.mu.Lock()
	n.Nick = newNick
	n.mu.Unlock()

	n.Broadcast(n.CreateHandshake(), "")
	n.Events <- Event{Type: "system", Data: "Nick actualizado a " + newNick}
}
func (n *Node) CreateHandshake() protocol.Message {
	knownPeers := []protocol.PeerInfo{}
	n.mu.RLock()
	count := 0
	for id, nick := range n.knownPeers {
		if id == n.ID {
			continue
		}
		if count >= n.KnownPeersLimit {
			break
		}
		knownPeers = append(knownPeers, protocol.PeerInfo{NodeID: id, Nick: nick})
		count++
	}
	n.mu.RUnlock()

	handshake := protocol.HandshakePayload{
		NodeID:        n.ID,
		Nick:          n.Nick,
		PublicKey:     n.PublicKey,
		KnownPeers:    knownPeers,
		SignPublicKey: n.SignPublicKey,
	}

	signData, _ := json.Marshal(handshake)
	signature, err := crypto.SignMessage(n.SignPrivateKey, signData)
	if err == nil {
		handshake.Signature = signature
	}

	payload, _ := json.Marshal(handshake)

	return protocol.Message{
		Type:      "handshake",
		FromID:    n.ID,
		FromNick:  n.Nick,
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}
}

func (n *Node) CreateChatMessage(text string) protocol.Message {
	payload, _ := json.Marshal(protocol.ChatPayload{
		Message: text,
	})

	return protocol.Message{
		ID:        uuid.New().String(),
		Type:      "chat",
		FromID:    n.ID,
		FromNick:  n.Nick,
		Timestamp: time.Now().Unix(),
		TTL:       8,
		Payload:   payload,
	}
}

func (n *Node) CreateAck(messageID, toID string) protocol.Message {
	payload, _ := json.Marshal(protocol.AckPayload{
		MessageID: messageID,
	})

	return protocol.Message{
		ID:        uuid.New().String(),
		Type:      "ack",
		FromID:    n.ID,
		FromNick:  n.Nick,
		ToID:      &toID,
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}
}

func (n *Node) updateRoutesFromHandshake(fromPeer *peer.Peer, payload protocol.HandshakePayload) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.routes[fromPeer.ID] = fromPeer.ID
	for _, info := range payload.KnownPeers {
		if info.NodeID == n.ID || info.NodeID == fromPeer.ID {
			continue
		}
		if _, exists := n.peers[info.NodeID]; exists {
			continue
		}
		if len(n.routes) >= n.RoutesLimit {
			// skip adding more routes when limit reached
			continue
		}
		if _, exists := n.routes[info.NodeID]; !exists {
			n.routes[info.NodeID] = fromPeer.ID
		}
		if info.Nick != "" {
			n.knownPeers[info.NodeID] = info.Nick
		}
	}
}

func (n *Node) RecordHistory(msg protocol.Message) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.history = append(n.history, msg)
}

func (n *Node) History() []protocol.Message {
	n.mu.RLock()
	defer n.mu.RUnlock()

	historyCopy := make([]protocol.Message, len(n.history))
	copy(historyCopy, n.history)
	return historyCopy
}
