package community

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func createOwned(t *testing.T, s *Store) Agent {
	v := action(t, s, Action{Kind: "agent", Agent: Agent{Name: "Nova", Handle: "nova", Role: "写作", Prompt: "整理社区观点并提出建议"}})
	return v.Agents[len(v.Agents)-1]
}
func TestAgentEditAndScheduler(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	g := createOwned(t, s)
	g.Name = "Nova edited"
	g.Prompt = "根据社区讨论提出具体建议"
	action(t, s, Action{Kind: "agent-edit", ID: g.ID, Agent: g})
	for _, a := range []Action{{Kind: "agent-edit", ID: "atlas", Agent: g}, {Kind: "agent-schedule", ID: "atlas", Agent: Agent{ScheduleMinutes: 60, SchedulePrompt: "test", ScheduleTopic: "广场"}}, {Kind: "agent-schedule", ID: g.ID, Agent: Agent{ScheduleMinutes: 0}}} {
		r, _ := s.Dispatch(context.Background(), a)
		if r.Error == "" {
			t.Fatal("invalid ownership/configuration accepted")
		}
	}
	action(t, s, Action{Kind: "agent-schedule", ID: g.ID, Agent: Agent{ScheduleEnabled: true, ScheduleMinutes: 60, SchedulePrompt: "总结近期讨论", ScheduleTopic: "产品思考"}})
	g = s.Snapshot().Agents[len(s.state.Agents)-1]
	due, _ := time.Parse(time.RFC3339, g.NextRunAt)
	s.queueDue(due.Add(-time.Second))
	if len(s.Snapshot().Runs) != 0 {
		t.Fatal("ran early")
	}
	s.queueDue(due)
	s.queueDue(due.Add(24 * time.Hour))
	if len(s.Snapshot().Runs) != 1 {
		t.Fatal("duplicate queued run")
	}
	r := s.Snapshot().Runs[0]
	if !r.Scheduled || r.TaskContent != "总结近期讨论" {
		t.Fatal("task snapshot missing")
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	g = s.Snapshot().Agents[len(s.state.Agents)-1]
	if g.Name != "Nova edited" || !g.ScheduleEnabled || g.SchedulePrompt != "总结近期讨论" {
		t.Fatal("edit or schedule not persisted")
	}
	s.queueDue(due.Add(48 * time.Hour))
	if len(s.Snapshot().Runs) != 2 {
		t.Fatal("missed executions not coalesced")
	}
	action(t, s, Action{Kind: "agent-schedule", ID: g.ID, Agent: Agent{ScheduleMinutes: 60, SchedulePrompt: "改为新任务", ScheduleTopic: "广场"}})
	if s.Snapshot().Runs[1].Status != "cancelled" {
		t.Fatal("pending old task not cancelled")
	}
	s.queueDue(due.Add(72 * time.Hour))
	if len(s.Snapshot().Runs) != 2 {
		t.Fatal("paused schedule executed")
	}
}
func TestScheduledTaskPublishesAgentPost(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	g := createOwned(t, s)
	action(t, s, Action{Kind: "settings", Settings: Settings{Model: "test"}, AppKey: "test-key"})
	action(t, s, Action{Kind: "agent-schedule", ID: g.ID, Agent: Agent{ScheduleMinutes: 60, SchedulePrompt: "提出一个设计问题", ScheduleTopic: "设计灵感"}})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), "提出一个设计问题") {
			t.Error("missing task")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"今天讨论：如何减少操作步骤？\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	s.providerBaseURL = server.URL + "/v1"
	action(t, s, Action{Kind: "agent-run-now", ID: g.ID})
	s.runNext(context.Background())
	v := s.Snapshot()
	if v.Runs[0].Status != "done" || v.Runs[0].PostID != v.Posts[0].ID || v.Posts[0].Author != g.ID || v.Posts[0].Topic != "设计灵感" {
		t.Fatalf("scheduled publication failed: %+v", v.Runs)
	}
	if s.runNext(context.Background()) {
		t.Fatal("scheduled output recursively triggered tasks")
	}
}
