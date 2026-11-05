package node

import (
	"net"
	"sync"
	"testing"
	"time"

	"p2p-chat/peer"
	"p2p-chat/protocol"
)

func connectTestPeers(t *testing.T, left, right *Node) (*peer.Peer, *peer.Peer) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for test peers: %v", err)
	}
	defer listener.Close()

	accepted := make(chan net.Conn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		accepted <- conn
	}()

	leftConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial test peer: %v", err)
	}
	var rightConn net.Conn
	select {
	case rightConn = <-accepted:
	case err := <-acceptErr:
		leftConn.Close()
		t.Fatalf("accept test peer: %v", err)
	case <-time.After(2 * time.Second):
		leftConn.Close()
		t.Fatal("accept test peer timed out")
	}

	leftPeer := peer.NewPeer(leftConn, left)
	rightPeer := peer.NewPeer(rightConn, right)
	leftPeer.RateLimit = 1000
	rightPeer.RateLimit = 1000
	leftPeer.RateWindow = time.Second
	rightPeer.RateWindow = time.Second

	go leftPeer.ReadLoop()
	go rightPeer.ReadLoop()

	leftPeer.Send(left.CreateHandshake())
	rightPeer.Send(right.CreateHandshake())

	waitFor(t, 2*time.Second, func() bool {
		return hasPeer(left, right.ID) && hasPeer(right, left.ID)
	})

	return leftPeer, rightPeer
}

func hasPeer(n *Node, id string) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	_, ok := n.peers[id]
	return ok
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition was not satisfied before timeout")
}

func closeTestPeers(peers ...*peer.Peer) {
	for _, p := range peers {
		if p != nil {
			p.Close()
		}
	}
}

func TestHandshakeEstablishesPeerAndSharedKey(t *testing.T) {
	alice := NewNode("node-alice", "Alice")
	bob := NewNode("node-bob", "Bob")

	leftPeer, rightPeer := connectTestPeers(t, alice, bob)
	defer closeTestPeers(leftPeer, rightPeer)

	alice.mu.RLock()
	alicePeer := alice.peers[bob.ID]
	alice.mu.RUnlock()
	bob.mu.RLock()
	bobPeer := bob.peers[alice.ID]
	bob.mu.RUnlock()

	if alicePeer == nil || bobPeer == nil {
		t.Fatal("expected both nodes to register the connected peer")
	}
	if !alicePeer.Identified || !bobPeer.Identified {
		t.Fatal("expected both peers to complete identification")
	}
	if len(alicePeer.SharedKey) == 0 || len(bobPeer.SharedKey) == 0 {
		t.Fatal("expected handshake to establish shared keys")
	}
	if string(alicePeer.SharedKey) != string(bobPeer.SharedKey) {
		t.Fatal("expected both peers to derive the same shared key")
	}
}

func TestDirectedMessageRoutesThroughIntermediateNode(t *testing.T) {
	alice := NewNode("node-alice", "Alice")
	bob := NewNode("node-bob", "Bob")
	carol := NewNode("node-carol", "Carol")

	abLeft, abRight := connectTestPeers(t, alice, bob)
	bcLeft, bcRight := connectTestPeers(t, bob, carol)
	defer closeTestPeers(abLeft, abRight, bcLeft, bcRight)
	routed := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		bob.mu.RLock()
		_, bobKnowsCarol := bob.peers[carol.ID]
		bob.mu.RUnlock()
		alice.mu.RLock()
		nextHop, aliceKnowsCarol := alice.routes[carol.ID]
		alice.mu.RUnlock()
		if bobKnowsCarol && aliceKnowsCarol && nextHop == bob.ID {
			routed = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !routed {
		bob.mu.RLock()
		bobPeers := make([]string, 0, len(bob.peers))
		for id := range bob.peers {
			bobPeers = append(bobPeers, id)
		}
		bob.mu.RUnlock()
		alice.mu.RLock()
		aliceRoutes := map[string]string{}
		for id, via := range alice.routes {
			aliceRoutes[id] = via
		}
		alice.mu.RUnlock()
		t.Fatalf("routing did not converge: bob peers=%v, alice routes=%v", bobPeers, aliceRoutes)
	}

	message := alice.CreateChatMessage("mensaje enrutado")
	message.ToID = stringPtr(carol.ID)
	alice.SendTo(carol.ID, message)

	waitFor(t, 2*time.Second, func() bool {
		for _, received := range carol.History() {
			if received.ID == message.ID && received.FromID == alice.ID {
				return true
			}
		}
		return false
	})
}

func stringPtr(value string) *string {
	return &value
}

func TestRecordHistoryIsSafeUnderConcurrency(t *testing.T) {
	n := NewNode("node-history", "History")
	const messages = 500

	var wg sync.WaitGroup
	for i := 0; i < messages; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			n.RecordHistory(protocol.Message{ID: string(rune(i + 1))})
		}(i)
	}
	wg.Wait()

	if got := len(n.History()); got != messages {
		t.Fatalf("expected %d history entries, got %d", messages, got)
	}
}
