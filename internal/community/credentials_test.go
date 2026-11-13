package community

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncryptedCredentialsSurviveRestart(t *testing.T) {
	t.Setenv("OMNITHREADS_APP_KEY", "")
	t.Setenv("DEEPSEEK_API_KEY", "")
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	action(t, s, Action{Kind: "settings", Settings: Settings{Model: "deepseek-reasoner", DisplayName: "Saved user"}, AppKey: "persist-default-secret"})
	action(t, s, Action{Kind: "agent-settings", ID: "atlas", Agent: Agent{CustomModel: true, Model: "deepseek-chat"}, AppKey: "persist-agent-secret"})
	rows, err := s.db.Query(context.Background(), "SELECT owner, encrypted FROM credentials")
	if err != nil || len(rows.Values) != 2 {
		t.Fatal("credentials not stored")
	}
	for _, row := range rows.Values {
		if _, err = s.openKey("wrong-owner", row[1].([]byte)); err == nil {
			t.Fatal("ciphertext not bound to owner")
		}
	}
	s.Close()
	for _, name := range []string{"community.sqlite", "credentials.key"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "persist-default-secret") || strings.Contains(string(b), "persist-agent-secret") {
			t.Fatal("plaintext persisted")
		}
		info, _ := os.Stat(filepath.Join(dir, name))
		if info.Mode().Perm()&0077 != 0 {
			t.Fatal("file readable by other users")
		}
	}
	t.Setenv("OMNITHREADS_APP_KEY", "environment-fallback")
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.key != "persist-default-secret" || s.agentKeys["atlas"] != "persist-agent-secret" || s.state.Settings.DisplayName != "Saved user" {
		t.Fatal("configuration not restored")
	}
	action(t, s, Action{Kind: "agent-settings", ID: "atlas"})
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.agentKeys["atlas"] != "" || s.state.Agents[0].CustomModel {
		t.Fatal("independent credentials not cleared")
	}
	s.Close()
	if err = os.WriteFile(filepath.Join(dir, "credentials.key"), make([]byte, 32), 0600); err != nil {
		t.Fatal(err)
	}
	if s, err = Open(dir); err == nil {
		s.Close()
		t.Fatal("wrong encryption key silently accepted")
	}
}
