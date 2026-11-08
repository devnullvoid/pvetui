package commandrunner

import (
	"bytes"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

func startAgentSSHServer(t *testing.T, key gossh.PublicKey, user, forwardAddr string) string {
	t.Helper()
	hostKey, _ := generateTestKey(t)
	signer, err := gossh.ParsePrivateKey(hostKey)
	if err != nil {
		t.Fatal(err)
	}
	config := &gossh.ServerConfig{
		PublicKeyCallback: func(meta gossh.ConnMetadata, candidate gossh.PublicKey) (*gossh.Permissions, error) {
			if meta.User() != user || !bytes.Equal(candidate.Marshal(), key.Marshal()) {
				return nil, errors.New("unexpected test SSH identity")
			}
			return nil, nil
		},
	}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	connections := make(map[net.Conn]struct{})
	var wg sync.WaitGroup
	closing := false
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			if closing {
				mu.Unlock()
				_ = conn.Close()
				return
			}
			connections[conn] = struct{}{}
			wg.Add(1)
			mu.Unlock()
			go func() {
				defer wg.Done()
				defer func() {
					_ = conn.Close()
					mu.Lock()
					delete(connections, conn)
					mu.Unlock()
				}()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				server, channels, requests, err := gossh.NewServerConn(conn, config)
				if err != nil {
					return
				}
				defer func() { _ = server.Close() }()
				_ = conn.SetDeadline(time.Time{})
				go gossh.DiscardRequests(requests)
				for channel := range channels {
					if channel.ChannelType() != "direct-tcpip" || forwardAddr == "" {
						_ = channel.Reject(gossh.UnknownChannelType, "test server only supports forwarding")
						continue
					}
					forwardAgentSSHChannel(channel, forwardAddr)
				}
			}()
		}
	}()
	t.Cleanup(func() {
		mu.Lock()
		closing = true
		for conn := range connections {
			_ = conn.Close()
		}
		mu.Unlock()
		_ = listener.Close()
		wg.Wait()
	})
	return listener.Addr().String()
}

func forwardAgentSSHChannel(request gossh.NewChannel, addr string) {
	// nolint:gosec // G704: addr is the test-created loopback SSH listener, never the channel's payload.
	upstream, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		_ = request.Reject(gossh.ConnectionFailed, "test target unavailable")
		return
	}
	defer func() { _ = upstream.Close() }()
	channel, requests, err := request.Accept()
	if err != nil {
		return
	}
	defer func() { _ = channel.Close() }()
	go gossh.DiscardRequests(requests)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(upstream, channel)
		_ = upstream.Close()
	}()
	_, _ = io.Copy(channel, upstream)
	_ = channel.Close()
	<-done
}

func splitAgentSSHAddress(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	return host, port
}

func TestSSHAgentDial(t *testing.T) {
	for _, tc := range []struct {
		name   string
		jump   bool
		reject bool
	}{
		{name: "direct"},
		{name: "jump", jump: true},
		{name: "direct retry", reject: true},
		{name: "jump retry", jump: true, reject: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key, pub := generateTestKey(t)
			cleanup := startTestAgent(t, key)
			defer cleanup()
			t.Setenv("HOME", t.TempDir())
			addr := startAgentSSHServer(t, pub, "target-user", "")
			host, port := splitAgentSSHAddress(t, addr)
			client := NewSSHClient(SSHClientConfig{Username: "target-user", Port: port, Timeout: 2 * time.Second})
			if tc.jump {
				jumpAddr := startAgentSSHServer(t, pub, "jump-user", addr)
				jumpHost, jumpPort := splitAgentSSHAddress(t, jumpAddr)
				client.jumpHost = JumpHostConfig{Addr: jumpHost, Port: jumpPort, User: "jump-user"}
			}
			if tc.reject {
				if tc.jump {
					client.jumpHost.User = "rejected-user"
				} else {
					client.username = "rejected-user"
				}
				if conn, closeJump, err := client.dialHost(host); err == nil {
					_ = conn.Close()
					if closeJump != nil {
						closeJump()
					}
					t.Fatal("expected authentication failure")
				}
				client.username = "target-user"
				if tc.jump {
					client.jumpHost.User = "jump-user"
				}
			}
			for attempt := range 2 {
				conn, closeJump, err := client.dialHost(host)
				if err != nil {
					t.Fatalf("agent handshake attempt %d: %v", attempt, err)
				}
				_ = conn.Close()
				if closeJump != nil {
					closeJump()
				}
			}
		})
	}
}
