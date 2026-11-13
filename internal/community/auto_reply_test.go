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

func TestDirectReplyRouting(t *testing.T) {
	for _, tc := range []struct {
		name, parent, text string
		count              int
		automatic          bool
	}{
		{"agent comment", "r1", "你觉得怎么规划比较好", 1, true},
		{"agent post", "", "请详细解释", 1, true},
		{"explicit mention deduplicated", "r1", "@muse 请详细解释", 1, false},
		{"another mention", "r1", "@atlas 也帮忙看看", 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			s, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			v := action(t, s, Action{Kind: "reply", ID: "welcome-research", ReplyID: tc.parent, Content: tc.text})
			if len(v.Runs) != tc.count || v.Runs[0].AutoReply != tc.automatic {
				t.Fatalf("incorrect routing: %+v", v.Runs)
			}
			child := v.Posts[0].Replies[len(v.Posts[0].Replies)-1]
			if v.Runs[0].ReplyID != child.ID {
				t.Fatal("missing target comment")
			}
			// Replying to a human without a mention must not summon the post author.
			v = action(t, s, Action{Kind: "reply", ID: "welcome-research", ReplyID: child.ID, Content: "补充一下"})
			if len(v.Runs) != tc.count {
				t.Fatal("human reply summoned agent")
			}
			s.Close()
			s, err = Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if s.Snapshot().Runs[0].AutoReply != tc.automatic {
				t.Fatal("automatic routing lost after restart")
			}
		})
	}
}

func TestAutomaticReplyDecisionAndNoLoop(t *testing.T) {
	for _, skip := range []bool{false, true} {
		t.Run(fmt.Sprint(skip), func(t *testing.T) {
			output := "我建议先明确目标，再规划投入。"
			input := "你觉得怎么规划比较好"
			if skip {
				output = noReply
				input = "谢谢，明白了"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Messages []map[string]any `json:"messages"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				text := fmt.Sprint(req.Messages)
				if !strings.Contains(text, noReply) || !strings.Contains(text, input) || !strings.Contains(text, "设计过程也能被看见") {
					t.Error("missing decision policy or target context")
				}
				w.Header().Set("Content-Type", "text/event-stream")
				payload, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]string{"content": output}}}})
				fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", payload)
			}))
			defer server.Close()
			dir := t.TempDir()
			s, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			s.providerBaseURL = server.URL + "/v1"
			action(t, s, Action{Kind: "settings", Settings: Settings{Model: "test"}, AppKey: "test-key"})
			v := action(t, s, Action{Kind: "reply", ID: "welcome-research", ReplyID: "r1", Content: input})
			before := len(v.Posts[0].Replies)
			target := v.Runs[0].ReplyID
			s.runNext(context.Background())
			v = s.Snapshot()
			if len(v.Runs) != 1 || s.runNext(context.Background()) {
				t.Fatal("agent response started another run")
			}
			if skip {
				if v.Runs[0].Status != "skipped" || len(v.Posts[0].Replies) != before {
					t.Fatal("unnecessary reply published")
				}
			} else {
				replies := v.Posts[0].Replies
				last := replies[len(replies)-1]
				if v.Runs[0].Status != "done" || last.ParentID != target || last.Author != "muse" || last.Content != output {
					t.Fatal("response missing or incorrectly linked")
				}
			}
			s.Close()
			s, err = Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if s.Snapshot().Runs[0].Status != v.Runs[0].Status {
				t.Fatal("decision not persisted")
			}
		})
	}
}
