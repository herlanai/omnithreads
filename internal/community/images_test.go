package community

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImageSettingsAndContentPersistence(t *testing.T) {
	t.Setenv("ARK_API_KEY", "")
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	v := action(t, s, Action{Kind: "settings", Settings: Settings{Model: "chat", ImageModel: "image-test"}, ARKKey: "private-ark-key"})
	if !v.Settings.HasARKKey {
		t.Fatal("missing configured status")
	}
	raw, _ := json.Marshal(v)
	if strings.Contains(string(raw), "private-ark-key") {
		t.Fatal("credential leaked")
	}
	v = action(t, s, Action{Kind: "agent", Agent: Agent{Name: "Painter", Handle: "painter", Prompt: "Draw helpful illustrations", ImageGeneration: true}})
	ag := v.Agents[len(v.Agents)-1]
	action(t, s, Action{Kind: "agent-settings", ID: ag.ID, Agent: Agent{ImageGeneration: false}})
	if s.state.Agents[len(s.state.Agents)-1].ImageGeneration {
		t.Fatal("disable failed")
	}
	action(t, s, Action{Kind: "agent-edit", ID: ag.ID, Agent: ag})
	image := GeneratedImage{Data: avatarFixture(t, 32), Size: "32x32"}
	s.state.Posts[0].Images = []GeneratedImage{image}
	s.state.Posts[0].Replies[0].Images = []GeneratedImage{image}
	if err = s.save(); err != nil {
		t.Fatal(err)
	}
	action(t, s, Action{Kind: "settings", Settings: Settings{Model: "chat", ImageModel: "image-test"}})
	bad, _ := s.Dispatch(context.Background(), Action{Kind: "settings", Settings: Settings{Model: "chat"}, ARKKey: "bad\nkey"})
	if bad.Error == "" || s.arkKey != "private-ark-key" {
		t.Fatal("bad key changed credentials")
	}
	s.Close()
	db, _ := os.ReadFile(filepath.Join(dir, "community.sqlite"))
	if strings.Contains(string(db), "private-ark-key") {
		t.Fatal("plaintext key stored")
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.arkKey != "private-ark-key" || !s.state.Settings.HasARKKey || s.state.Settings.ImageModel != "image-test" || !s.state.Agents[len(s.state.Agents)-1].ImageGeneration {
		t.Fatal("image config lost")
	}
	if s.state.Posts[0].Images[0].Data != image.Data || s.state.Posts[0].Replies[0].Images[0].Data != image.Data {
		t.Fatal("images lost")
	}
	view := s.Snapshot()
	if strings.HasPrefix(view.Posts[0].Images[0].Data, "data:") {
		t.Fatal("large image leaked into snapshot")
	}
	req := httptest.NewRequest(http.MethodGet, view.Posts[0].Images[0].Data, nil)
	rec := httptest.NewRecorder()
	s.ServeImage(rec, req)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatal("local media not available")
	}
	if "data:image/png;base64,"+base64.StdEncoding.EncodeToString(rec.Body.Bytes()) != image.Data {
		t.Fatal("media bytes changed")
	}

}

func TestImageGenerationThroughHarness(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			t.Setenv("ARK_API_KEY", "environment-must-not-enable-tools")
			fixture := avatarFixture(t, 32)
			pngBytes, _ := base64.StdEncoding.DecodeString(strings.SplitN(fixture, ",", 2)[1])
			arkCalls := 0
			var ark *httptest.Server
			ark = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/result.png" {
					w.Header().Set("Content-Type", "image/png")
					w.Write(pngBytes)
					return
				}
				arkCalls++
				if r.Header.Get("Authorization") != "Bearer private-image-key" {
					t.Error("wrong image credential")
				}
				var request map[string]any
				json.NewDecoder(r.Body).Decode(&request)
				if request["model"] != "image-fixture" {
					t.Error("wrong image model")
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"data":[{"url":%q,"size":"32x32"}]}`, ark.URL+"/result.png")
			}))
			defer ark.Close()
			steps := 0
			chat := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				steps++
				var request map[string]any
				json.NewDecoder(r.Body).Decode(&request)
				raw, _ := json.Marshal(request)
				if strings.Contains(string(raw), "private-image-key") {
					t.Error("image key leaked to chat")
				}
				tools, _ := request["tools"].([]any)
				if (len(tools) > 0) != enabled {
					t.Error("incorrect tool capability")
				}
				w.Header().Set("Content-Type", "text/event-stream")
				if enabled && steps == 1 {
					fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"draw-1\",\"type\":\"function\",\"function\":{\"name\":\"ark_generate_image\",\"arguments\":\"{\\\"prompt\\\":\\\"Draw a bird\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n")
				} else {
					fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Done\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
				}
			}))
			defer chat.Close()
			result, err := completeConfigured(context.Background(), Settings{Model: "chat", ImageModel: "image-fixture"}, "chat-key", "private-image-key", Agent{Name: "Painter", ImageGeneration: enabled}, Post{Content: "Draw a bird"}, chat.URL, false, ark.URL, ark.Client())
			if err != nil {
				t.Fatal(err)
			}
			if enabled {
				if arkCalls != 1 || len(result.Images) != 1 || result.Images[0].Data != fixture {
					t.Fatalf("missing image: calls=%d images=%d", arkCalls, len(result.Images))
				}
			} else if arkCalls != 0 || len(result.Images) != 0 {
				t.Fatal("disabled agent generated image")
			}
		})
	}
}

func TestGeneratedImageDownloadBlocksPrivateHosts(t *testing.T) {
	client := imageClient()
	req, _ := http.NewRequest(http.MethodGet, "https://127.0.0.1/result.png", nil)
	if _, err := client.Do(req); err == nil {
		t.Fatal("private host accepted")
	}
}
