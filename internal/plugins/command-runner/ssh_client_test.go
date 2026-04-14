package commandrunner

import (
	"crypto/rand"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"

	"crypto/ed25519"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// generateTestKey creates an ed25519 key pair and returns the private key PEM bytes
// and the corresponding ssh.PublicKey.
func generateTestKey(t *testing.T) (pemBytes []byte, pub gossh.PublicKey) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}

	privPEM, err := gossh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	pemBytes = pem.EncodeToMemory(privPEM)
	return pemBytes, signer.PublicKey()
}

// startTestAgent starts an in-memory SSH agent on a temp unix socket, adds the
// provided key, sets SSH_AUTH_SOCK, and returns a cleanup function.
func startTestAgent(t *testing.T, keyPEM []byte) (cleanup func()) {
	t.Helper()

	keyring := agent.NewKeyring()

	privPEM, _ := pem.Decode(keyPEM)
	if privPEM == nil {
		t.Fatal("failed to decode PEM block")
	}
	privKey, err := gossh.ParseRawPrivateKey(keyPEM)
	if err != nil {
		t.Fatalf("parse private key: %v", err)
	}
	if err := keyring.Add(agent.AddedKey{PrivateKey: privKey}); err != nil {
		t.Fatalf("add key to agent: %v", err)
	}

	sock := filepath.Join(t.TempDir(), "agent.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen on agent socket: %v", err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // listener closed
			}
			go func() {
				_ = agent.ServeAgent(keyring, conn)
			}()
		}
	}()

	prev := os.Getenv("SSH_AUTH_SOCK")
	_ = os.Setenv("SSH_AUTH_SOCK", sock)

	return func() {
		_ = ln.Close()
		if prev == "" {
			_ = os.Unsetenv("SSH_AUTH_SOCK")
		} else {
			_ = os.Setenv("SSH_AUTH_SOCK", prev)
		}
	}
}

func newTestClient() *SSHClientImpl {
	return &SSHClientImpl{timeout: 5}
}

// TestLoadSSHKeys_AgentOnly verifies that signers are loaded from a running SSH agent.
func TestLoadSSHKeys_AgentOnly(t *testing.T) {
	keyPEM, _ := generateTestKey(t)
	cleanup := startTestAgent(t, keyPEM)
	defer cleanup()

	// Point HOME at an empty dir so no file-based keys interfere.
	t.Setenv("HOME", t.TempDir())

	signers, err := newTestClient().loadSSHKeys("")
	if err != nil {
		t.Fatalf("expected signers from agent, got error: %v", err)
	}
	if len(signers) == 0 {
		t.Fatal("expected at least one signer from agent")
	}
}

// TestLoadSSHKeys_ExplicitKeyfile verifies that a configured keyfile is loaded directly.
func TestLoadSSHKeys_ExplicitKeyfile(t *testing.T) {
	keyPEM, _ := generateTestKey(t)

	dir := t.TempDir()
	keyPath := filepath.Join(dir, "test_key")
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	// No SSH agent.
	t.Setenv("SSH_AUTH_SOCK", "")

	signers, err := newTestClient().loadSSHKeys(keyPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signers) != 1 {
		t.Fatalf("expected 1 signer, got %d", len(signers))
	}
}

// TestLoadSSHKeys_StandardPathFallback verifies keys are found at ~/.ssh/id_ed25519.
func TestLoadSSHKeys_StandardPathFallback(t *testing.T) {
	keyPEM, _ := generateTestKey(t)

	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "id_ed25519"), keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SSH_AUTH_SOCK", "")

	signers, err := newTestClient().loadSSHKeys("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signers) == 0 {
		t.Fatal("expected signer from standard path")
	}
}

// TestLoadSSHKeys_AgentAndKeyfileCombined verifies that agent and file signers are merged.
func TestLoadSSHKeys_AgentAndKeyfileCombined(t *testing.T) {
	agentKeyPEM, agentPub := generateTestKey(t)
	fileKeyPEM, filePub := generateTestKey(t)

	cleanup := startTestAgent(t, agentKeyPEM)
	defer cleanup()

	dir := t.TempDir()
	keyPath := filepath.Join(dir, "file_key")
	if err := os.WriteFile(keyPath, fileKeyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	signers, err := newTestClient().loadSSHKeys(keyPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signers) != 2 {
		t.Fatalf("expected 2 signers (agent + file), got %d", len(signers))
	}

	// Verify both public keys are present.
	pubKeys := map[string]bool{
		string(agentPub.Marshal()): false,
		string(filePub.Marshal()):  false,
	}
	for _, s := range signers {
		key := string(s.PublicKey().Marshal())
		if _, ok := pubKeys[key]; ok {
			pubKeys[key] = true
		}
	}
	for k, found := range pubKeys {
		if !found {
			t.Errorf("expected public key %x not found in signers", k)
		}
	}
}

// TestLoadSSHKeys_NoAuthAvailable verifies an error is returned when no auth sources exist.
func TestLoadSSHKeys_NoAuthAvailable(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("HOME", t.TempDir()) // empty home — no keys

	_, err := newTestClient().loadSSHKeys("")
	if err == nil {
		t.Fatal("expected error when no auth available, got nil")
	}
}

// TestLoadSSHKeys_MissingExplicitKeyfile verifies an error when a configured keyfile is absent.
func TestLoadSSHKeys_MissingExplicitKeyfile(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")

	_, err := newTestClient().loadSSHKeys("/nonexistent/path/key")
	if err == nil {
		t.Fatal("expected error for missing keyfile, got nil")
	}
}

// TestLoadSSHKeys_ExplicitKeyfileTakesPrecedence verifies that when an explicit keyPath
// is provided, only that key is loaded and the standard path key is not included.
func TestLoadSSHKeys_ExplicitKeyfileTakesPrecedence(t *testing.T) {
	explicitKeyPEM, explicitPub := generateTestKey(t)
	stdKeyPEM, _ := generateTestKey(t)

	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	_ = os.MkdirAll(sshDir, 0o700)
	_ = os.WriteFile(filepath.Join(sshDir, "id_ed25519"), stdKeyPEM, 0o600)

	explicitPath := filepath.Join(t.TempDir(), "explicit_key")
	_ = os.WriteFile(explicitPath, explicitKeyPEM, 0o600)

	t.Setenv("HOME", home)
	t.Setenv("SSH_AUTH_SOCK", "")

	signers, err := newTestClient().loadSSHKeys(explicitPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signers) != 1 {
		t.Fatalf("expected exactly 1 signer when explicit keyPath given, got %d", len(signers))
	}
	if string(signers[0].PublicKey().Marshal()) != string(explicitPub.Marshal()) {
		t.Error("explicit keyfile signer does not match expected public key")
	}
}
