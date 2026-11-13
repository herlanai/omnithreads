package community

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func avatarFixture(t *testing.T, size int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes())
}
func TestAvatarPersistenceAndRemoval(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	avatar := avatarFixture(t, 64)
	v := action(t, s, Action{Kind: "agent", Agent: Agent{Name: "Picture", Handle: "picture", Role: "Design", Prompt: "Help with design", Avatar: avatar}})
	g := v.Agents[len(v.Agents)-1]
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	g = s.state.Agents[len(s.state.Agents)-1]
	if g.Avatar != avatar {
		t.Fatal("avatar not restored")
	}
	g.Name = "Edited"
	action(t, s, Action{Kind: "agent-edit", ID: g.ID, Agent: g})
	if s.state.Agents[len(s.state.Agents)-1].Avatar != avatar {
		t.Fatal("profile edit lost avatar")
	}
	g.Avatar = avatarFixture(t, 32)
	action(t, s, Action{Kind: "agent-edit", ID: g.ID, Agent: g})
	if s.state.Agents[len(s.state.Agents)-1].Avatar != g.Avatar {
		t.Fatal("avatar replacement lost")
	}
	g.Avatar = ""
	action(t, s, Action{Kind: "agent-edit", ID: g.ID, Agent: g})
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.state.Agents[len(s.state.Agents)-1].Avatar != "" {
		t.Fatal("avatar removal not saved")
	}
}
func TestAvatarValidation(t *testing.T) {
	for _, value := range []string{"https://example.com/avatar.png", "data:image/svg+xml;base64,PHN2Zz4=", "data:image/png;base64,bad", strings.Repeat("x", 65537), avatarFixture(t, 257), strings.Replace(avatarFixture(t, 32), "image/png", "image/jpeg", 1)} {
		if validateAvatar(value) == nil {
			t.Fatal("invalid image accepted")
		}
	}
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	g := s.state.Agents[0]
	g.Avatar = avatarFixture(t, 32)
	result, _ := s.Dispatch(context.Background(), Action{Kind: "agent-edit", ID: g.ID, Agent: g})
	if result.Error == "" || s.state.Agents[0].Avatar != "" {
		t.Fatal("non-owned agent avatar changed")
	}
}
