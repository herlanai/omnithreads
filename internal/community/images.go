package community

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	dshgo "github.com/herlanai/omnigo/omniui/plugins/dsh-go"
	_ "golang.org/x/image/webp"
)

// Images are saved with the post/reply, so provider URL expiry doesn't remove them.
type GeneratedImage struct {
	Data string `json:"data"`
	Size string `json:"size"`
}
type completion struct {
	Text   string
	Images []GeneratedImage
}

func imageClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if !ip.IP.IsGlobalUnicast() || ip.IP.IsPrivate() || ip.IP.IsLoopback() || ip.IP.IsLinkLocalUnicast() {
				return nil, errors.New("invalid image host")
			}
		}
		for _, ip := range ips {
			conn, e := (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if e == nil {
				return conn, nil
			}
			err = e
		}
		if err == nil {
			err = errors.New("image host unavailable")
		}
		return nil, err
	}
	return &http.Client{Transport: transport, Timeout: 60 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || req.URL.Scheme != "https" {
			return errors.New("invalid image redirect")
		}
		return nil
	}}
}

func saveGeneratedImages(ctx context.Context, images []dshgo.GeneratedImage, client *http.Client) ([]GeneratedImage, error) {
	if len(images) > 4 {
		return nil, errors.New("生成图片数量过多，请每次最多生成 4 张")
	}
	out := make([]GeneratedImage, 0, len(images))
	for _, generated := range images {
		u, err := url.Parse(generated.URL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
			return nil, errors.New("生成图片地址无效")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		res, err := client.Do(req)
		if err != nil {
			return nil, errors.New("图片已生成，但保存失败，请稍后重试")
		}
		data, readErr := io.ReadAll(io.LimitReader(res.Body, 20<<20+1))
		res.Body.Close()
		if res.StatusCode != 200 || readErr != nil || len(data) > 20<<20 {
			return nil, errors.New("图片已生成，但保存失败，请稍后重试")
		}
		config, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > 40_000_000 {
			return nil, errors.New("生成图片格式或尺寸无效")
		}
		if format != "png" && format != "jpeg" && format != "webp" {
			return nil, errors.New("生成图片格式或尺寸无效")
		}
		if _, _, err = image.Decode(bytes.NewReader(data)); err != nil {
			return nil, errors.New("生成图片格式或尺寸无效")
		}
		out = append(out, GeneratedImage{Data: "data:image/" + format + ";base64," + base64.StdEncoding.EncodeToString(data), Size: generated.Size})
	}
	return out, nil
}

func imageSystem(enabled bool) string {
	if enabled {
		return "你可以使用图片生成工具满足用户的绘图请求，每次最多生成 4 张。仅在任务要求图片时生成，失败后不要重复尝试。图片会由社区自动展示，不要在正文重复嵌入图片链接。"
	}
	return "你只能基于当前讨论提供文本成果，没有图片生成能力。"
}

func stripGeneratedLinks(text string, images []dshgo.GeneratedImage) string {
	for _, image := range images {
		text = regexp.MustCompile(`!?\[[^\]]*\]\(`+regexp.QuoteMeta(image.URL)+`\)`).ReplaceAllString(text, "")
		text = strings.ReplaceAll(text, image.URL, "")
	}
	return text
}

// RPC snapshots carry stable local URLs, never multi-megabyte image payloads.
// Original image bytes remain in the SQLite-backed state.
func (s *Store) publicState() State {
	view := s.state
	view.Posts = append([]Post{}, s.state.Posts...)
	for i := range view.Posts {
		p := &view.Posts[i]
		p.Images = publicImages(p.Images, p.ID, "")
		p.Replies = append([]Reply{}, p.Replies...)
		for j := range p.Replies {
			r := &p.Replies[j]
			r.Images = publicImages(r.Images, p.ID, r.ID)
		}
	}
	return clone(view)
}
func publicImages(images []GeneratedImage, postID, replyID string) []GeneratedImage {
	if replyID == "" {
		replyID = "post"
	}
	out := make([]GeneratedImage, len(images))
	for i, img := range images {
		out[i] = GeneratedImage{Data: "/media/" + postID + "/" + replyID + "/" + strconv.Itoa(i), Size: img.Size}
	}
	return out
}
func (s *Store) ServeImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/media/"), "/")
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}
	index, err := strconv.Atoi(parts[2])
	if err != nil || index < 0 {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	p := s.post(parts[0])
	var data string
	if p != nil {
		images := p.Images
		if parts[1] != "post" {
			images = nil
			for _, reply := range p.Replies {
				if reply.ID == parts[1] {
					images = reply.Images
					break
				}
			}
		}
		if index < len(images) {
			data = images[index].Data
		}
	}
	s.mu.Unlock()
	if data == "" {
		http.NotFound(w, r)
		return
	}
	header, encoded, ok := strings.Cut(data, ",")
	if !ok {
		http.NotFound(w, r)
		return
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	mime := strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64")
	w.Header().Set("Content-Type", mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	if r.Method == http.MethodGet {
		w.Write(raw)
	}
}
