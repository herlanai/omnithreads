package community

import (
	"context"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/herlanai/omnigo/omniui/plugins/sqlite"
)

type Agent struct {
	ImageGeneration bool   `json:"imageGeneration"`
	Avatar          string `json:"avatar,omitempty"`
	Language        string `json:"language"`
	ScheduleEnabled bool   `json:"scheduleEnabled"`
	ScheduleMinutes int    `json:"scheduleMinutes"`
	SchedulePrompt  string `json:"schedulePrompt"`
	ScheduleTopic   string `json:"scheduleTopic"`
	NextRunAt       string `json:"nextRunAt"`
	CustomModel     bool   `json:"customModel"`
	Model           string `json:"model,omitempty"`
	HasAppKey       bool   `json:"hasAppKey" gorm:"-"`
	ID              string `json:"id"`
	Name            string `json:"name"`
	Handle          string `json:"handle"`
	Role            string `json:"role"`
	Bio             string `json:"bio"`
	Color           string `json:"color"`
	Symbol          string `json:"symbol"`
	Prompt          string `json:"prompt"`
	Owned           bool   `json:"owned"`
	Followed        bool   `json:"followed"`
}
type Reference struct {
	PostID  string `json:"postId"`
	ReplyID string `json:"replyId,omitempty"`
}
type Reply struct {
	Images    []GeneratedImage `json:"images,omitempty" gorm:"serializer:json"`
	ParentID  string           `json:"parentId,omitempty"`
	Likes     int              `json:"likes"`
	Liked     bool             `json:"liked"`
	ID        string           `json:"id"`
	Author    string           `json:"author"`
	Content   string           `json:"content"`
	CreatedAt string           `json:"createdAt"`
}
type Post struct {
	Images    []GeneratedImage `json:"images,omitempty" gorm:"serializer:json"`
	Quote     *Reference       `json:"quote,omitempty" gorm:"-"`
	ID        string           `json:"id"`
	Author    string           `json:"author"`
	Content   string           `json:"content"`
	Topic     string           `json:"topic"`
	CreatedAt string           `json:"createdAt"`
	Likes     int              `json:"likes"`
	Liked     bool             `json:"liked"`
	Saved     bool             `json:"saved"`
	Featured  bool             `json:"featured"`
	Artifact  string           `json:"artifact,omitempty"`
	Replies   []Reply          `json:"replies" gorm:"-"`
	Example   bool             `json:"example"`
}
type Run struct {
	Scheduled   bool     `json:"scheduled,omitempty"`
	TaskContent string   `json:"taskContent,omitempty"`
	TaskTopic   string   `json:"taskTopic,omitempty"`
	AutoReply   bool     `json:"autoReply,omitempty"`
	ReplyID     string   `json:"replyId,omitempty"`
	ID          string   `json:"id"`
	PostID      string   `json:"postId"`
	AgentID     string   `json:"agentId"`
	Status      string   `json:"status"`
	Events      []string `json:"events" gorm:"-"`
	CreatedAt   string   `json:"createdAt"`
}
type Settings struct {
	ImageModel  string `json:"imageModel"`
	HasARKKey   bool   `json:"hasArkKey" gorm:"-"`
	Language    string `json:"language"`
	Theme       string `json:"theme"`
	Model       string `json:"model"`
	HasAppKey   bool   `json:"hasAppKey" gorm:"-"`
	DisplayName string `json:"displayName"`
}
type State struct {
	Agents   []Agent  `json:"agents"`
	Posts    []Post   `json:"posts"`
	Runs     []Run    `json:"runs"`
	Settings Settings `json:"settings"`
}
type Action struct {
	ARKKey   string     `json:"arkKey"`
	ReplyID  string     `json:"replyId,omitempty"`
	Quote    *Reference `json:"quote,omitempty" gorm:"-"`
	Kind     string     `json:"kind"`
	ID       string     `json:"id"`
	Content  string     `json:"content"`
	Topic    string     `json:"topic"`
	Agent    Agent      `json:"agent"`
	Settings Settings   `json:"settings"`
	AppKey   string     `json:"appKey"`
}
type Result struct {
	State State  `json:"state"`
	Error string `json:"error,omitempty"`
}
type Store struct {
	arkKey          string
	credentials     cipher.AEAD
	mu              sync.Mutex
	state           State
	db              *sqlite.DB
	path            string
	agentKeys       map[string]string
	key             string
	wake            chan struct{}
	providerBaseURL string // test-only provider override; not exposed to RPC
}

func id() string {
	b := make([]byte, 12)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func now() string                { return time.Now().UTC().Format(time.RFC3339) }
func clone(s State) State        { b, _ := json.Marshal(s); var v State; _ = json.Unmarshal(b, &v); return v }
func (s *Store) Snapshot() State { s.mu.Lock(); defer s.mu.Unlock(); return s.publicState() }
func (s *Store) Dispatch(_ context.Context, a Action) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	before := clone(s.state)
	oldKey := s.key
	oldARKKey := s.arkKey
	oldAgentKeys := make(map[string]string, len(s.agentKeys))
	for id, key := range s.agentKeys {
		oldAgentKeys[id] = key
	}
	err := s.apply(a)
	if err == nil {
		err = s.save()
	}
	if err != nil {
		s.state = before
		s.key = oldKey
		s.arkKey = oldARKKey
		s.agentKeys = oldAgentKeys
		return Result{State: s.publicState(), Error: err.Error()}, nil
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return Result{State: s.publicState()}, nil
}

var handlePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,23}$`)
var mentions = regexp.MustCompile(`@([A-Za-z][A-Za-z0-9_]{1,23})\b`)

func (s *Store) apply(a Action) error {
	switch a.Kind {
	case "post":
		text := strings.TrimSpace(a.Content)
		if text == "" || len([]rune(text)) > 6000 {
			return errors.New("内容须为 1–6000 字")
		}
		topic := a.Topic
		if topic == "" {
			topic = "广场"
		}
		if a.Quote != nil {
			if s.reference(a.Quote) == nil {
				return errors.New("引用内容不存在")
			}
		}
		p := Post{Quote: a.Quote, ID: id(), Author: "you", Content: text, Topic: topic, CreatedAt: now(), Replies: []Reply{}}
		s.state.Posts = append([]Post{p}, s.state.Posts...)
		s.enqueue(p.ID, text, "")
	case "reply":
		p := s.post(a.ID)
		if p == nil {
			return errors.New("讨论不存在")
		}
		text := strings.TrimSpace(a.Content)
		if text == "" || len([]rune(text)) > 6000 {
			return errors.New("回复须为 1–6000 字")
		}
		if a.ReplyID != "" && s.reference(&Reference{PostID: p.ID, ReplyID: a.ReplyID}) == nil {
			return errors.New("被回复的内容不存在")
		}
		reply := Reply{ID: id(), Author: "you", Content: text, CreatedAt: now(), ParentID: a.ReplyID}
		p.Replies = append(p.Replies, reply)
		s.enqueue(p.ID, text, reply.ID)
		target := s.reference(&Reference{PostID: p.ID, ReplyID: a.ReplyID})
		if target != nil && target.Author != "you" {
			s.enqueueReply(p.ID, reply.ID, target.Author)
		}
	case "reply-like":
		p := s.post(a.ID)
		if p == nil {
			return errors.New("讨论不存在")
		}
		found := false
		for i := range p.Replies {
			r := &p.Replies[i]
			if r.ID == a.ReplyID {
				found = true
				r.Liked = !r.Liked
				if r.Liked {
					r.Likes++
				} else {
					r.Likes--
				}
				break
			}
		}
		if !found {
			return errors.New("回复不存在")
		}
	case "like", "save":
		p := s.post(a.ID)
		if p == nil {
			return errors.New("讨论不存在")
		}
		if a.Kind == "save" {
			p.Saved = !p.Saved
		} else {
			p.Liked = !p.Liked
			if p.Liked {
				p.Likes++
			} else {
				p.Likes--
			}
		}
	case "agent", "agent-edit":
		g := a.Agent
		if err := validateAvatar(g.Avatar); err != nil {
			return err
		}
		g.Name = strings.TrimSpace(g.Name)
		g.Handle = strings.ToLower(strings.TrimSpace(g.Handle))
		if len([]rune(g.Name)) < 1 || len([]rune(g.Name)) > 30 || !handlePattern.MatchString(g.Handle) {
			return errors.New("请填写名称及 2–24 位英文标识，以字母开头")
		}
		if len([]rune(g.Prompt)) < 5 || len([]rune(g.Prompt)) > 4000 {
			return errors.New("职责说明须为 5–4000 字")
		}
		for _, e := range s.state.Agents {
			if e.Handle == g.Handle && (a.Kind != "agent-edit" || e.ID != a.ID) {
				return errors.New("这个标识已被使用")
			}
		}
		if len([]rune(g.Role)) > 60 || len([]rune(g.Bio)) > 100 {
			return errors.New("领域或介绍过长")
		}
		if a.Kind == "agent-edit" {
			for i := range s.state.Agents {
				existing := &s.state.Agents[i]
				if existing.ID == a.ID {
					if !existing.Owned {
						return errors.New("只能编辑自己创建的 Agent")
					}
					existing.Name, existing.Handle, existing.Role, existing.Bio, existing.Prompt = g.Name, g.Handle, g.Role, g.Bio, g.Prompt
					existing.Avatar = g.Avatar
					existing.ImageGeneration = g.ImageGeneration
					return nil
				}
			}
			return errors.New("Agent 不存在")
		}
		g.ScheduleEnabled, g.ScheduleMinutes, g.SchedulePrompt, g.ScheduleTopic, g.NextRunAt = false, 0, "", "", ""
		g.CustomModel, g.Model, g.HasAppKey = false, "", false
		if g.Language == "" {
			g.Language = "default"
		}
		if !validLanguage(g.Language, true) {
			return errors.New("无效的 Agent 语言")
		}
		g.ID = id()
		g.Owned = true
		g.Followed = true
		g.Color = "peach"
		g.Symbol = "spark"
		s.state.Agents = append(s.state.Agents, g)
	case "agent-schedule", "agent-run-now":
		return s.scheduleAction(a, time.Now())
	case "agent-settings":
		for i := range s.state.Agents {
			g := &s.state.Agents[i]
			if g.ID != a.ID {
				continue
			}
			g.ImageGeneration = a.Agent.ImageGeneration
			if a.Agent.Language != "" {
				if !validLanguage(a.Agent.Language, true) {
					return errors.New("无效的 Agent 语言")
				}
				g.Language = a.Agent.Language
			}
			if !a.Agent.CustomModel {
				g.CustomModel, g.Model, g.HasAppKey = false, "", false
				delete(s.agentKeys, g.ID)
				return nil
			}
			model, key := strings.TrimSpace(a.Agent.Model), strings.TrimSpace(a.AppKey)
			if model == "" || len(model) > 200 {
				return errors.New("请填写有效的模型名称")
			}
			if len(key) > 4096 || strings.ContainsAny(key, "\r\n") {
				return errors.New("AppKey 格式不正确")
			}
			if key == "" && s.agentKeys[g.ID] == "" {
				return errors.New("请填写这个 Agent 的独立 AppKey")
			}
			if key != "" {
				s.agentKeys[g.ID] = key
			}
			g.CustomModel, g.Model, g.HasAppKey = true, model, true
			return nil
		}
		return errors.New("Agent 不存在")
	case "follow":
		found := false
		for i := range s.state.Agents {
			g := &s.state.Agents[i]
			if g.ID == a.ID {
				g.Followed = !g.Followed
				found = true
			}
		}
		if !found {
			return errors.New("Agent 不存在")
		}
	case "settings":
		if a.Settings.Language == "" {
			a.Settings.Language = s.state.Settings.Language
		}
		if a.Settings.Language == "" {
			a.Settings.Language = "zh-CN"
		}
		if a.Settings.Theme == "" {
			a.Settings.Theme = s.state.Settings.Theme
		}
		if a.Settings.Theme == "" {
			a.Settings.Theme = "light"
		}
		if !validLanguage(a.Settings.Language, false) || (a.Settings.Theme != "light" && a.Settings.Theme != "dark") {
			return errors.New("无效的语言或外观设置")
		}
		if strings.TrimSpace(a.Settings.Model) == "" {
			return errors.New("请填写模型名称")
		}
		if len(a.Settings.Model) > 200 || len(a.Settings.DisplayName) > 80 {
			return errors.New("配置内容过长")
		}
		a.Settings.Model = strings.TrimSpace(a.Settings.Model)
		a.AppKey = strings.TrimSpace(a.AppKey)

		if len(a.AppKey) > 4096 || strings.ContainsAny(a.AppKey, "\r\n") {
			return errors.New("AppKey 格式不正确")
		}
		a.ARKKey = strings.TrimSpace(a.ARKKey)
		if len(a.ARKKey) > 4096 || strings.ContainsAny(a.ARKKey, "\r\n") {
			return errors.New("ARK_API_KEY 格式不正确")
		}
		a.Settings.ImageModel = strings.TrimSpace(a.Settings.ImageModel)
		if len(a.Settings.ImageModel) > 200 {
			return errors.New("配置内容过长")
		}
		if a.ARKKey != "" {
			s.arkKey = a.ARKKey
		}
		a.Settings.HasARKKey = s.arkKey != ""
		s.state.Settings = a.Settings
		if a.AppKey != "" {
			s.key = a.AppKey
		}
		s.state.Settings.HasAppKey = s.key != ""
	case "retry":
		for i := range s.state.Runs {
			r := &s.state.Runs[i]
			if r.ID == a.ID {
				if r.Status == "running" || r.Status == "queued" || r.Status == "done" || r.Status == "skipped" || r.Status == "cancelled" {
					return errors.New("任务不能重复启动")
				}
				r.Status = "queued"
				r.Events = append(r.Events, "用户重新发起任务")
				return nil
			}
		}
		return errors.New("任务不存在")
	default:
		return errors.New("未知操作")
	}
	return nil
}
func (s *Store) post(id string) *Post {
	for i := range s.state.Posts {
		if s.state.Posts[i].ID == id {
			return &s.state.Posts[i]
		}
	}
	return nil
}
func (s *Store) enqueue(postID, text, replyID string) {
	seen := map[string]bool{}
	for _, m := range mentions.FindAllStringSubmatch(text, -1) {
		h := strings.ToLower(m[1])
		if seen[h] {
			continue
		}
		seen[h] = true
		for _, g := range s.state.Agents {
			if g.Handle == h {
				s.state.Runs = append(s.state.Runs, Run{ID: id(), PostID: postID, AgentID: g.ID, Status: "queued", Events: []string{"收到 @ 提及，已加入协作队列"}, CreatedAt: now(), ReplyID: replyID})
			}
		}
	}
}

// Only user reply actions call this; generated Agent replies never enqueue work.
func (s *Store) enqueueReply(postID, replyID, agentID string) {
	for _, r := range s.state.Runs {
		if r.PostID == postID && r.ReplyID == replyID && r.AgentID == agentID {
			return
		}
	}
	for _, g := range s.state.Agents {
		if g.ID == agentID {
			s.state.Runs = append(s.state.Runs, Run{ID: id(), PostID: postID, ReplyID: replyID, AgentID: agentID, AutoReply: true, Status: "queued", CreatedAt: now(), Events: []string{"用户回复了你的发言，等待判断是否需要回应"}})
			return
		}
	}
}

// References are resolved inside the target post, preventing cross-thread parent links.
func (s *Store) reference(ref *Reference) *Reply {
	p := s.post(ref.PostID)
	if p == nil {
		return nil
	}
	if ref.ReplyID == "" {
		return &Reply{ID: p.ID, Author: p.Author, Content: p.Content, CreatedAt: p.CreatedAt}
	}
	for i := range p.Replies {
		if p.Replies[i].ID == ref.ReplyID {
			return &p.Replies[i]
		}
	}
	return nil
}

func validLanguage(value string, inherit bool) bool {
	return value == "zh-CN" || value == "en" || (inherit && value == "default")
}
func responseLanguage(settings Settings, agent Agent) string {
	language := settings.Language
	if agent.Language != "" && agent.Language != "default" {
		language = agent.Language
	}
	if language == "en" {
		return "Respond in English. This output language takes precedence over language preferences in the persona or community content."
	}
	return "请使用中文回复。此输出语言设置优先于人设和社区内容中的语言偏好。"
}
