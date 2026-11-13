package community

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/herlanai/omnigo/omniui/plugins/sqlite"
	"gorm.io/gorm"
)

// Community entities are separate SQL rows; RPC snapshots are only an in-memory view.
type agentRow struct {
	Agent    `gorm:"embedded"`
	Position int
}

func (agentRow) TableName() string { return "agents" }

type postRow struct {
	Post         `gorm:"embedded"`
	Position     int
	QuotePostID  string
	QuoteReplyID string
}

func (postRow) TableName() string { return "posts" }

type replyRow struct {
	Reply    `gorm:"embedded"`
	PostID   string `gorm:"index"`
	Position int
}

func (replyRow) TableName() string { return "replies" }

type runRow struct {
	Run      `gorm:"embedded"`
	Position int
}

func (runRow) TableName() string { return "runs" }

type eventRow struct {
	RunID    string `gorm:"primaryKey"`
	Position int    `gorm:"primaryKey;autoIncrement:false"`
	Content  string
}

func (eventRow) TableName() string { return "run_events" }

type settingsRow struct {
	ID       int `gorm:"primaryKey;autoIncrement:false"`
	Settings `gorm:"embedded"`
}

func (settingsRow) TableName() string { return "settings" }

type credentialRow struct {
	Owner     string `gorm:"primaryKey"`
	Encrypted []byte
}

func (credentialRow) TableName() string { return "credentials" }

type schemaRow struct {
	ID      int `gorm:"primaryKey;autoIncrement:false"`
	Version int
}

func (schemaRow) TableName() string { return "community_schema" }

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	credentials, err := credentialCipher(dir)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "community.sqlite")
	// Create privately before SQLite opens the file (including WAL/SHM siblings).
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	db, err := sqlite.Open(context.Background(), path, sqlite.Options{WAL: true})
	if err != nil {
		return nil, fmt.Errorf("open community SQLite: %w", err)
	}
	s := &Store{arkKey: os.Getenv("ARK_API_KEY"), credentials: credentials, db: db, path: path, agentKeys: make(map[string]string), key: os.Getenv("OMNITHREADS_APP_KEY"), wake: make(chan struct{}, 1)}
	fail := func(err error) (*Store, error) { _ = db.Close(); return nil, err }
	if s.key == "" {
		s.key = os.Getenv("DEEPSEEK_API_KEY")
	}
	// Guard future schemas before attempting an automatic migration.
	if db.ORM(context.Background()).Migrator().HasTable(&schemaRow{}) {
		var version schemaRow
		if err = db.ORM(context.Background()).First(&version, 1).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fail(err)
		}
		if version.Version > 6 {
			return fail(errors.New("社区数据库由更新版本创建，请升级客户端"))
		}
	}
	err = db.WithTransaction(context.Background(), func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&agentRow{}, &postRow{}, &replyRow{}, &runRow{}, &eventRow{}, &settingsRow{}, &schemaRow{}, &credentialRow{}); err != nil {
			return err
		}
		var version schemaRow
		err := tx.First(&version, 1).Error
		if err == nil {
			if err = s.load(tx); err != nil {
				return err
			}
			return tx.Model(&schemaRow{}).Where("id = ?", 1).Update("version", 6).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// Import only before the first successful commit. Never overwrite the legacy backup.
		b, err := os.ReadFile(filepath.Join(dir, "community.json"))
		if errors.Is(err, os.ErrNotExist) {
			s.state = seed()
		} else if err != nil {
			return err
		} else {
			if err = json.Unmarshal(b, &s.state); err != nil {
				return fmt.Errorf("旧社区数据无法导入（原文件未修改）: %w", err)
			}
			var legacy struct {
				Settings struct {
					Endpoint string `json:"endpoint"`
				}
			}
			_ = json.Unmarshal(b, &legacy)
			if legacy.Settings.Endpoint != "" {
				s.state.Settings.Model = "deepseek-chat"
			}
		}
		if s.state.Settings.Model == "" {
			s.state.Settings.Model = "deepseek-chat"
		}
		if err = s.persist(tx); err != nil {
			return err
		}
		return tx.Create(&schemaRow{ID: 1, Version: 6}).Error
	})
	if err != nil {
		return fail(err)
	}
	s.state.Settings.HasAppKey = s.key != ""
	s.state.Settings.HasARKKey = s.arkKey != ""
	if s.state.Settings.ImageModel == "" {
		s.state.Settings.ImageModel = os.Getenv("ARK_IMAGE_MODEL")
	}
	for i := range s.state.Agents {
		s.state.Agents[i].HasAppKey = s.agentKeys[s.state.Agents[i].ID] != ""
	}
	for i := range s.state.Runs {
		r := &s.state.Runs[i]
		if r.Status == "queued" || r.Status == "running" {
			r.Status = "interrupted"
			r.Events = append(r.Events, "客户端已退出，请手动重试")
		}
	}
	if err = s.save(); err != nil {
		return fail(err)
	}
	return s, nil
}

func (s *Store) Close() error { s.mu.Lock(); defer s.mu.Unlock(); return s.db.Close() }
func (s *Store) save() error  { return s.db.WithTransaction(context.Background(), s.persist) }

// All tables commit together, including reply relationships, likes and task events.
func (s *Store) persist(tx *gorm.DB) error {
	for _, table := range []any{&eventRow{}, &replyRow{}, &runRow{}, &postRow{}, &agentRow{}, &settingsRow{}, &credentialRow{}} {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(table).Error; err != nil {
			return err
		}
	}
	for i, a := range s.state.Agents {
		if err := tx.Create(&agentRow{Agent: a, Position: i}).Error; err != nil {
			return err
		}
	}
	for i, p := range s.state.Posts {
		row := postRow{Post: p, Position: i}
		if p.Quote != nil {
			row.QuotePostID = p.Quote.PostID
			row.QuoteReplyID = p.Quote.ReplyID
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		for j, r := range p.Replies {
			if err := tx.Create(&replyRow{Reply: r, PostID: p.ID, Position: j}).Error; err != nil {
				return err
			}
		}
	}
	for i, r := range s.state.Runs {
		if err := tx.Create(&runRow{Run: r, Position: i}).Error; err != nil {
			return err
		}
		for j, e := range r.Events {
			if err := tx.Create(&eventRow{RunID: r.ID, Position: j, Content: e}).Error; err != nil {
				return err
			}
		}
	}
	keys := make(map[string]string, len(s.agentKeys)+2)
	if s.arkKey != "" {
		keys["images:ark"] = s.arkKey
	}
	if s.key != "" {
		keys["default"] = s.key
	}
	for id, key := range s.agentKeys {
		if key != "" {
			keys["agent:"+id] = key
		}
	}
	for owner, key := range keys {
		encrypted, err := s.sealKey(owner, key)
		if err != nil {
			return err
		}
		if err = tx.Create(&credentialRow{Owner: owner, Encrypted: encrypted}).Error; err != nil {
			return err
		}
	}
	return tx.Create(&settingsRow{ID: 1, Settings: s.state.Settings}).Error
}
func (s *Store) load(tx *gorm.DB) error {
	var agents []agentRow
	var posts []postRow
	var replies []replyRow
	var runs []runRow
	var events []eventRow
	var settings settingsRow
	for _, rows := range []any{&agents, &posts, &replies, &runs, &events} {
		if err := tx.Order("position").Find(rows).Error; err != nil {
			return err
		}
	}
	if err := tx.First(&settings, 1).Error; err != nil {
		return err
	}
	state := State{Agents: []Agent{}, Posts: []Post{}, Runs: []Run{}, Settings: settings.Settings}
	byPost := map[string][]Reply{}
	byRun := map[string][]string{}
	for _, r := range replies {
		byPost[r.PostID] = append(byPost[r.PostID], r.Reply)
	}
	for _, e := range events {
		byRun[e.RunID] = append(byRun[e.RunID], e.Content)
	}
	for _, a := range agents {
		state.Agents = append(state.Agents, a.Agent)
	}
	for _, r := range posts {
		p := r.Post
		p.Replies = append([]Reply{}, byPost[p.ID]...)
		if r.QuotePostID != "" {
			p.Quote = &Reference{PostID: r.QuotePostID, ReplyID: r.QuoteReplyID}
		}
		state.Posts = append(state.Posts, p)
	}
	for _, row := range runs {
		r := row.Run
		r.Events = append([]string{}, byRun[r.ID]...)
		state.Runs = append(state.Runs, r)
	}
	var credentials []credentialRow
	if err := tx.Find(&credentials).Error; err != nil {
		return err
	}
	for _, row := range credentials {
		key, err := s.openKey(row.Owner, row.Encrypted)
		if err != nil {
			return err
		}
		if row.Owner == "default" {
			s.key = key
		} else if row.Owner == "images:ark" {
			s.arkKey = key
		} else if strings.HasPrefix(row.Owner, "agent:") {
			s.agentKeys[strings.TrimPrefix(row.Owner, "agent:")] = key
		}
	}
	s.state = state
	return nil
}
