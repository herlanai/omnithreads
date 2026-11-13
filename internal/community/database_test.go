package community

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSQLiteMigrationAndAuthoritativeReopen(t *testing.T) {
	t.Setenv("OMNITHREADS_APP_KEY", "")
	t.Setenv("DEEPSEEK_API_KEY", "")
	dir := t.TempDir()
	legacy := seed()
	legacy.Settings = Settings{DisplayName: "迁移用户", Model: "deepseek-reasoner"}
	legacy.Agents[0].CustomModel = true
	legacy.Agents[0].Model = "custom-model"
	legacy.Posts[0].Saved = true
	legacy.Posts[0].Liked = true
	legacy.Posts[0].Replies = append(legacy.Posts[0].Replies, Reply{ID: "child", ParentID: "r1", Author: "you", Content: "**完整保留**", Likes: 3, Liked: true})
	legacy.Posts[1].Quote = &Reference{PostID: legacy.Posts[0].ID, ReplyID: "child"}
	legacy.Runs = []Run{{ID: "run", PostID: legacy.Posts[0].ID, AgentID: "atlas", ReplyID: "child", Status: "done", Events: []string{"读取上下文", "完成"}}}
	b, _ := json.Marshal(legacy)
	path := filepath.Join(dir, "community.json")
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(legacy, s.Snapshot()) {
		t.Fatal("migration changed community data")
	}
	rows, err := s.db.Query(context.Background(), "SELECT content, parent_id, likes FROM replies WHERE id = ?", "child")
	if err != nil || len(rows.Values) != 1 || rows.Values[0][0] != "**完整保留**" || rows.Values[0][1] != "r1" {
		t.Fatalf("missing relational reply: %v %v", rows, err)
	}
	preserved, err := os.ReadFile(path)
	if err != nil || string(preserved) != string(b) {
		t.Fatal("legacy backup changed")
	}
	action(t, s, Action{Kind: "post", Content: "SQLite 新讨论"})
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	header, err := os.ReadFile(filepath.Join(dir, "community.sqlite"))
	if err != nil || string(header[:16]) != "SQLite format 3\x00" {
		t.Fatal("not a SQLite database")
	}
	// Existing SQL content wins even when the old backup is no longer valid.
	if err = os.WriteFile(path, []byte("invalid legacy backup"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if len(s.Snapshot().Posts) != 5 || s.Snapshot().Posts[0].Content != "SQLite 新讨论" {
		t.Fatal("reimported stale JSON")
	}
}

func TestSQLiteFailedMigrationCanRetry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "community.json")
	original := []byte("{broken")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(dir); err == nil {
		s.Close()
		t.Fatal("corrupt JSON silently seeded")
	}
	b, _ := os.ReadFile(path)
	if string(b) != string(original) {
		t.Fatal("corrupt original modified")
	}
	legacy := seed()
	legacy.Agents = append(legacy.Agents, legacy.Agents[0])
	b, _ = json.Marshal(legacy)
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(dir); err == nil {
		s.Close()
		t.Fatal("duplicate IDs silently imported")
	}
	legacy = seed()
	b, _ = json.Marshal(legacy)
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if len(s.Snapshot().Agents) != 4 || len(s.Snapshot().Posts) != 4 {
		t.Fatal("partial migration persisted")
	}
}

func TestSQLiteWriteRollbackIsDurable(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	before := s.Snapshot()
	_, err = s.db.Execute(context.Background(), `CREATE TRIGGER fail_write BEFORE INSERT ON posts BEGIN SELECT RAISE(ABORT, 'test failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.Dispatch(context.Background(), Action{Kind: "post", Content: "must roll back"})
	if err != nil || r.Error == "" {
		t.Fatal("expected transaction failure")
	}
	if !reflect.DeepEqual(before, s.Snapshot()) {
		t.Fatal("memory did not roll back")
	}
	if _, err = s.db.Execute(context.Background(), "DROP TRIGGER fail_write"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if !reflect.DeepEqual(before, s.Snapshot()) {
		t.Fatal("database did not roll back")
	}
}
