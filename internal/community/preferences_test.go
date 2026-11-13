package community

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLanguageAndThemePersistence(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	action(t, s, Action{Kind: "settings", Settings: Settings{Model: "deepseek-chat", Language: "en", Theme: "dark"}})
	action(t, s, Action{Kind: "agent-settings", ID: "atlas", Agent: Agent{Language: "zh-CN"}})
	if !strings.Contains(responseLanguage(s.state.Settings, s.state.Agents[1]), "Respond in English") || !strings.Contains(responseLanguage(s.state.Settings, s.state.Agents[0]), "请使用中文") {
		t.Fatal("language precedence incorrect")
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.state.Settings.Theme != "dark" || s.state.Settings.Language != "en" || s.state.Agents[0].Language != "zh-CN" {
		t.Fatal("preferences lost")
	}
	action(t, s, Action{Kind: "agent-settings", ID: "atlas", Agent: Agent{Language: "en"}})
	action(t, s, Action{Kind: "settings", Settings: Settings{Model: "deepseek-chat", Language: "zh-CN", Theme: "light"}})
	if !strings.Contains(responseLanguage(s.state.Settings, s.state.Agents[0]), "Respond in English") {
		t.Fatal("independent language changed")
	}
	action(t, s, Action{Kind: "agent-settings", ID: "atlas", Agent: Agent{Language: "default"}})
	if !strings.Contains(responseLanguage(s.state.Settings, s.state.Agents[0]), "请使用中文") {
		t.Fatal("inheritance not restored")
	}
	for _, a := range []Action{{Kind: "settings", Settings: Settings{Model: "x", Language: "invalid"}}, {Kind: "settings", Settings: Settings{Model: "x", Theme: "invalid"}}, {Kind: "agent-settings", ID: "atlas", Agent: Agent{Language: "invalid"}}} {
		r, _ := s.Dispatch(context.Background(), a)
		if r.Error == "" {
			t.Fatal("invalid preference accepted")
		}
	}
	g := createOwned(t, s)
	action(t, s, Action{Kind: "agent-schedule", ID: g.ID, Agent: Agent{ScheduleEnabled: true, ScheduleMinutes: 60, SchedulePrompt: "写一篇文章", ScheduleTopic: "广场"}})
	action(t, s, Action{Kind: "agent-settings", ID: g.ID, Agent: Agent{Language: "en"}})
	if !s.state.Agents[len(s.state.Agents)-1].ScheduleEnabled {
		t.Fatal("language setting reset schedule")
	}
}
func TestLanguageReachesModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct{ Role, Content string }
		}
		json.NewDecoder(r.Body).Decode(&req)
		if len(req.Messages) == 0 || !strings.Contains(req.Messages[0].Content, "Respond in English") {
			t.Error("language missing from system prompt")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello community\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	_, err := complete(context.Background(), Settings{Model: "test", Language: "zh-CN"}, "test", Agent{Name: "Test", Language: "en"}, Post{Content: "你好"}, server.URL+"/v1", false)
	if err != nil {
		t.Fatal(err)
	}
}
