package community

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func action(t *testing.T, s *Store, a Action) State {
	t.Helper()
	r, e := s.Dispatch(context.Background(), a)
	if e != nil || r.Error != "" {
		t.Fatalf("action: %v %s", e, r.Error)
	}
	return r.State
}
func TestMentionBoundariesAndPersistence(t *testing.T) {
	d := t.TempDir()
	s, e := Open(d)
	if e != nil {
		t.Fatal(e)
	}
	v := action(t, s, Action{Kind: "post", Content: "@atlas @atlas @atlas_other @muse 帮我完善方案"})
	if len(v.Runs) != 2 {
		t.Fatalf("expected unique exact mentions, got %d", len(v.Runs))
	}
	p := v.Posts[0]
	action(t, s, Action{Kind: "save", ID: p.ID})
	s2, e := Open(d)
	if e != nil {
		t.Fatal(e)
	}
	v = s2.Snapshot()
	if !v.Posts[0].Saved || v.Posts[0].Content != p.Content {
		t.Fatal("content lost after reopen")
	}
	if v.Runs[0].Status != "interrupted" {
		t.Fatal("unfinished run not recovered")
	}
}
func TestSnapshotIsolationAndRollback(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	v := s.Snapshot()
	v.Posts[0].Replies[0].Content = "mutated"
	if s.Snapshot().Posts[0].Replies[0].Content == "mutated" {
		t.Fatal("snapshot aliases store")
	}
	if _, err := s.db.Execute(context.Background(), `CREATE TRIGGER fail_write BEFORE DELETE ON agents BEGIN SELECT RAISE(ABORT, 'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	r, _ := s.Dispatch(context.Background(), Action{Kind: "post", Content: "must not commit"})
	if r.Error == "" || len(s.Snapshot().Posts) != 4 {
		t.Fatal("failed save was not rolled back")
	}
}
func TestModelRunProducesDurableReplyWithoutSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Error("incorrect provider request")
		}
		var req struct {
			Messages []map[string]any `json:"messages"`
			Model    string           `json:"model"`
			Stream   bool             `json:"stream"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if len(req.Messages) < 2 || !strings.Contains(fmt.Sprint(req.Messages), "设计") || req.Model != "test" || !req.Stream {
			t.Error("missing thread context")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"这是可留存的设计成果\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	dir := t.TempDir()
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	s.providerBaseURL = server.URL + "/v1"
	action(t, s, Action{Kind: "settings", Settings: Settings{Model: "test", DisplayName: "Test"}, AppKey: "test-secret"})
	action(t, s, Action{Kind: "post", Content: "@muse 帮我设计社区"})
	if !s.runNext(context.Background()) {
		t.Fatal("run missing")
	}
	v := s.Snapshot()
	if v.Runs[0].Status != "done" || len(v.Posts[0].Replies) != 1 {
		t.Fatalf("run failed: %+v", v.Runs)
	}
	b, err := os.ReadFile(s.path + "-wal")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "test-secret") {
		t.Fatal("key persisted")
	}
	s2, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	if s2.Snapshot().Posts[0].Replies[0].Content != "这是可留存的设计成果" {
		t.Fatal("reply not persisted")
	}
}
func TestFailedAndUnconfiguredRuns(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	action(t, s, Action{Kind: "post", Content: "@atlas hi"})
	s.runNext(context.Background())
	v := s.Snapshot()
	if v.Runs[0].Status != "waiting" || len(v.Posts[0].Replies) > 0 {
		t.Fatal("must not fake output")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }))
	defer server.Close()
	s.providerBaseURL = server.URL
	action(t, s, Action{Kind: "settings", Settings: Settings{Model: "test"}, AppKey: "invalid-test-key"})
	action(t, s, Action{Kind: "retry", ID: v.Runs[0].ID})
	s.runNext(context.Background())
	v = s.Snapshot()
	if v.Runs[0].Status != "failed" || len(v.Posts[0].Replies) > 0 {
		t.Fatal("provider failure incorrectly completed")
	}
}
func TestAgentValidation(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	r, _ := s.Dispatch(context.Background(), Action{Kind: "agent", Agent: Agent{Name: "Atlas 2", Handle: "atlas", Prompt: "test prompt"}})
	if r.Error == "" {
		t.Fatal("duplicate handle accepted")
	}
	r, _ = s.Dispatch(context.Background(), Action{Kind: "settings", Settings: Settings{Model: "x"}})
	if r.Error != "" {
		t.Fatal("preferences must be editable without an AppKey")
	}
}

func TestSettingsMigrationAndKeyRetention(t *testing.T) {
	t.Setenv("OMNITHREADS_APP_KEY", "")
	t.Setenv("DEEPSEEK_API_KEY", "")
	dir := t.TempDir()
	old := seed()
	b, _ := json.Marshal(old)
	var raw map[string]any
	_ = json.Unmarshal(b, &raw)
	raw["settings"] = map[string]any{"endpoint": "https://old.example/v1", "model": "old-model", "displayName": "Halley"}
	b, _ = json.Marshal(raw)
	if e := os.WriteFile(filepath.Join(dir, "community.json"), b, 0600); e != nil {
		t.Fatal(e)
	}
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	if s.Snapshot().Settings.Model != "deepseek-chat" {
		t.Fatal("legacy model not reset")
	}
	action(t, s, Action{Kind: "settings", Settings: Settings{Model: "deepseek-reasoner"}, AppKey: "session-secret"})
	v := action(t, s, Action{Kind: "settings", Settings: Settings{Model: "deepseek-chat"}})
	if !v.Settings.HasAppKey || s.key != "session-secret" {
		t.Fatal("blank key did not retain credential")
	}
	s2, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	if !s2.Snapshot().Settings.HasAppKey || s2.key != "session-secret" {
		t.Fatal("saved key not restored")
	}
}

func TestReplyInteractionsAndQuotePersistence(t *testing.T) {
	dir := t.TempDir()
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	v := action(t, s, Action{Kind: "reply", ID: "welcome-research", ReplyID: "r1", Content: "**赞同**，@atlas 请继续分析"})
	child := v.Posts[0].Replies[len(v.Posts[0].Replies)-1]
	if child.ParentID != "r1" || v.Runs[0].ReplyID != child.ID {
		t.Fatal("reply/run parent association missing")
	}
	action(t, s, Action{Kind: "reply-like", ID: "welcome-research", ReplyID: child.ID})
	v = action(t, s, Action{Kind: "post", Content: "我对这条回复的补充", Quote: &Reference{PostID: "welcome-research", ReplyID: child.ID}})
	if len(v.Runs) != 2 {
		t.Fatal("quoted mention triggered duplicate agent task")
	}
	reopened, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	v = reopened.Snapshot()
	if v.Posts[0].Quote.ReplyID != child.ID {
		t.Fatal("quote lost")
	}
	r := reopened.reference(v.Posts[0].Quote)
	if r == nil || r.ParentID != "r1" || !r.Liked || r.Likes != 1 {
		t.Fatal("reply state lost")
	}
	for _, a := range []Action{
		{Kind: "reply", ID: "design-system", ReplyID: "r1", Content: "wrong thread"},
		{Kind: "reply-like", ID: "design-system", ReplyID: "r1"},
		{Kind: "post", Content: "invalid quote", Quote: &Reference{PostID: "design-system", ReplyID: "r1"}},
	} {
		out, _ := s.Dispatch(context.Background(), a)
		if out.Error == "" {
			t.Fatal("cross-thread reference accepted")
		}
	}
}

func TestAgentModelInheritanceAndOverrides(t *testing.T) {
	t.Setenv("OMNITHREADS_APP_KEY", "")
	t.Setenv("DEEPSEEK_API_KEY", "")
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	action(t, s, Action{Kind: "settings", Settings: Settings{Model: "default-one"}, AppKey: "default-secret"})
	check := func(index int, model, key string) {
		t.Helper()
		c, k := s.modelConfig(s.state.Agents[index])
		if c.Model != model || k != key {
			t.Fatal("incorrect effective configuration")
		}
	}
	check(0, "default-one", "default-secret")
	action(t, s, Action{Kind: "agent-settings", ID: "atlas", Agent: Agent{CustomModel: true, Model: "independent"}, AppKey: "private-secret"})
	action(t, s, Action{Kind: "settings", Settings: Settings{Model: "default-two"}, AppKey: "updated-secret"})
	check(0, "independent", "private-secret")
	check(1, "default-two", "updated-secret")
	action(t, s, Action{Kind: "agent-settings", ID: "atlas", Agent: Agent{CustomModel: true, Model: "independent-two"}})
	check(0, "independent-two", "private-secret")
	b, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, _ := json.Marshal(s.Snapshot())
	for _, secret := range []string{"default-secret", "updated-secret", "private-secret"} {
		if strings.Contains(string(b), secret) || strings.Contains(string(snapshot), secret) {
			t.Fatal("credential exposed")
		}
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := reopened.Snapshot().Agents[0]
	if !a.CustomModel || a.Model != "independent-two" || !a.HasAppKey {
		t.Fatal("incorrect restart state")
	}
	reopened.key = "another-default"
	_, key := reopened.modelConfig(a)
	if key != "private-secret" {
		t.Fatal("independent key not restored")
	}

	if _, err := s.db.Execute(context.Background(), `CREATE TRIGGER fail_write BEFORE DELETE ON agents BEGIN SELECT RAISE(ABORT, 'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	r, _ := s.Dispatch(context.Background(), Action{Kind: "agent-settings", ID: "atlas", Agent: Agent{CustomModel: true, Model: "bad-save"}, AppKey: "replacement"})
	if r.Error == "" {
		t.Fatal("expected save error")
	}
	check(0, "independent-two", "private-secret")
	if _, err := s.db.Execute(context.Background(), `DROP TRIGGER fail_write`); err != nil {
		t.Fatal(err)
	}
	for _, a := range []Action{
		{Kind: "agent-settings", ID: "missing"},
		{Kind: "agent-settings", ID: "muse", Agent: Agent{CustomModel: true, Model: "test"}},
		{Kind: "agent-settings", ID: "atlas", Agent: Agent{CustomModel: true, Model: "test"}, AppKey: "bad\nkey"},
	} {
		r, _ := s.Dispatch(context.Background(), a)
		if r.Error == "" {
			t.Fatal("invalid configuration accepted")
		}
	}
	action(t, s, Action{Kind: "agent-settings", ID: "atlas"})
	check(0, "default-two", "updated-secret")
	if s.agentKeys["atlas"] != "" {
		t.Fatal("independent key not cleared")
	}
}
