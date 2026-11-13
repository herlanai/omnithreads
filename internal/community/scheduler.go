package community

import (
	"errors"
	"log"
	"strings"
	"time"
)

func (s *Store) scheduledPending(agentID string) bool {
	for _, r := range s.state.Runs {
		if r.AgentID == agentID && r.Scheduled && (r.Status == "queued" || r.Status == "running" || r.Status == "waiting") {
			return true
		}
	}
	return false
}
func (s *Store) scheduleAction(a Action, t time.Time) error {
	for i := range s.state.Agents {
		g := &s.state.Agents[i]
		if g.ID != a.ID {
			continue
		}
		if !g.Owned {
			return errors.New("只能为自己创建的 Agent 设置任务")
		}
		if a.Kind == "agent-run-now" {
			if g.SchedulePrompt == "" {
				return errors.New("请先保存任务内容")
			}
			if s.scheduledPending(g.ID) {
				return errors.New("已有任务等待或正在执行")
			}
			s.addScheduled(*g, t)
			return nil
		}
		task := strings.TrimSpace(a.Agent.SchedulePrompt)
		minutes := a.Agent.ScheduleMinutes
		if len([]rune(task)) > 4000 || minutes < 15 || minutes > 10080 || task == "" {
			return errors.New("请填写 1–4000 字任务内容，间隔为 15–10080 分钟")
		}
		topic := a.Agent.ScheduleTopic
		valid := false
		for _, v := range []string{"广场", "产品思考", "设计灵感", "一起构建", "社区日常"} {
			if v == topic {
				valid = true
			}
		}
		if !valid {
			return errors.New("请选择发布话题")
		}
		g.ScheduleEnabled, g.ScheduleMinutes, g.SchedulePrompt, g.ScheduleTopic = a.Agent.ScheduleEnabled, minutes, task, topic
		g.NextRunAt = ""
		if g.ScheduleEnabled {
			g.NextRunAt = t.Add(time.Duration(minutes) * time.Minute).UTC().Format(time.RFC3339)
		}
		for j := range s.state.Runs {
			r := &s.state.Runs[j]
			if r.AgentID == g.ID && r.Scheduled && (r.Status == "queued" || r.Status == "waiting") {
				r.Status = "cancelled"
				r.Events = append(r.Events, "定时任务设置已更新，取消旧的待执行任务")
			}
		}
		return nil
	}
	return errors.New("Agent 不存在")
}
func (s *Store) addScheduled(g Agent, t time.Time) {
	s.state.Runs = append(s.state.Runs, Run{ID: id(), AgentID: g.ID, Scheduled: true, TaskContent: g.SchedulePrompt, TaskTopic: g.ScheduleTopic, Status: "queued", CreatedAt: t.UTC().Format(time.RFC3339), Events: []string{"定时任务已加入队列，完成后自动发布到「" + g.ScheduleTopic + "」"}})
}

// Run at most once after downtime; never replay every missed interval.
func (s *Store) queueDue(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	before := clone(s.state)
	changed := false
	for i := range s.state.Agents {
		g := &s.state.Agents[i]
		if !g.Owned || !g.ScheduleEnabled {
			continue
		}
		due, err := time.Parse(time.RFC3339, g.NextRunAt)
		if err != nil || t.Before(due) {
			continue
		}
		if s.scheduledPending(g.ID) {
			continue
		}
		s.addScheduled(*g, t)
		g.NextRunAt = t.Add(time.Duration(g.ScheduleMinutes) * time.Minute).UTC().Format(time.RFC3339)
		changed = true
	}
	if !changed {
		return
	}
	if err := s.save(); err != nil {
		s.state = before
		log.Printf("persist schedule: %v", err)
		return
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
}
