package renderer

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/dendec/zimlite/internal/article"
	"github.com/dendec/zimlite/internal/i18n"
	engine "github.com/dendec/zimlite/internal/servo"
	"github.com/veandco/go-sdl2/sdl"
)

type servoView struct {
	session                     *engine.Session
	texture                     *sdl.Texture
	width, height               uint32
	textureWidth, textureHeight int32
	links                       int
	selectedURL                 string
	tableScrollLeft             bool
	tableScrollRight            bool
	loading                     string
}

func servoHelper() string {
	if path := os.Getenv("KIWIX_SERVO_HELPER"); path != "" {
		return path
	}
	if dir := os.Getenv("KIWIX_APP_DIR"); dir != "" {
		return filepath.Join(dir, "servo", "run.sh")
	}
	exe, err := os.Executable()
	if err != nil {
		return "zimlite-servo-host"
	}
	launcher := filepath.Join(filepath.Dir(exe), "servo", "run.sh")
	if _, err := os.Stat(launcher); err == nil {
		return launcher
	}
	return filepath.Join(filepath.Dir(exe), "zimlite-servo-host")
}

// OpenArticle takes ownership of Source, including on failure. SDL retains the
// window and status bar; the helper owns only one article viewport.
func (r *Renderer) OpenArticle(source article.Source, entry string) error {
	helper := servoHelper()
	if _, err := os.Stat(helper); err != nil {
		_ = source.Close()
		return fmt.Errorf("servo helper unavailable: %w", err)
	}
	r.StopArticle()
	r.ClearCache()
	r.doc = nil
	r.treeItems = nil
	r.layout = PageLayout{}
	r.width, r.height = r.window.GetSize()
	width, height := r.servoViewport()
	session, err := engine.NewSession(helper, source, entry, width, height)
	if err != nil {
		return err
	}
	r.scrollY = 0
	r.selectedLink = -1
	r.hoveredLink = -1
	r.docTitle = entry
	r.servoError = nil
	r.servo = &servoView{session: session, width: width, height: height, loading: i18n.T(r.lang, "servo.loading")}
	r.SetStatusOverride(r.servo.loading)
	r.servoSend(engine.Command{Op: "zoom", Scale: float32(r.baseFontSize) / 16})
	r.sendServoTheme()
	return nil
}
func (r *Renderer) StopArticle() {
	if r.servo == nil {
		return
	}
	view := r.servo
	r.servo = nil
	r.servoError = nil
	view.session.Close()
	if view.texture != nil {
		view.texture.Destroy()
	}
	if r.GetStatusOverride() == view.loading {
		r.SetStatusOverride("")
	}
}
func (r *Renderer) ArticleActive() bool { return r.servo != nil }
func (r *Renderer) ArticleError() error { err := r.servoError; r.servoError = nil; return err }
func (r *Renderer) PollArticleNavigation() string {
	if r.servo == nil {
		return ""
	}
	select {
	case event := <-r.servo.session.Navigation:
		return event.URL
	default:
		return ""
	}
}

// ResolveArticleURL distinguishes loopback article links from external links.
func (r *Renderer) ResolveArticleURL(raw string) (string, string, bool) {
	if r.servo == nil {
		return "", "", false
	}
	base, err := url.Parse(r.servo.session.BaseURL())
	if err != nil {
		return "", "", false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", false
	}
	u = base.ResolveReference(u)
	if u.Scheme != base.Scheme || u.Host != base.Host || u.User != nil {
		return "", "", false
	}
	return strings.TrimPrefix(u.Path, "/"), u.Fragment, true
}
func (r *Renderer) ScrollToAnchor(anchor string) {
	r.servoSend(engine.Command{Op: "anchor", Text: anchor})
}

// ScrollTableHorizontally moves the selected link's table without moving the
// article. It returns false when no selected table can scroll in that direction.
func (r *Renderer) ScrollTableHorizontally(direction int32) bool {
	if r.servo == nil {
		return false
	}
	if direction == 0 || (direction < 0 && !r.servo.tableScrollLeft) || (direction > 0 && !r.servo.tableScrollRight) {
		return false
	}
	r.servoSend(engine.Command{Op: "table_scroll", Value: direction})
	return true
}

func (r *Renderer) servoSend(command engine.Command) {
	if r.servo != nil {
		if err := r.servo.session.Send(command); err != nil {
			r.servoError = err
		}
	}
}
func (r *Renderer) sendServoTheme() {
	theme := "light"
	if r.ThemeName() == "dark" {
		theme = "dark"
	}
	r.servoSend(engine.Command{Op: "theme", Text: theme})
}
func (r *Renderer) servoViewport() (uint32, uint32) {
	w, h := float64(r.width), float64(r.height-r.getStatusBarHeight())
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	scale := math.Min(1, math.Min(1280/w, 1280/h))
	scale = math.Min(scale, math.Sqrt(float64(engine.MaxPixels)/(w*h)))
	return uint32(math.Max(1, math.Floor(w*scale))), uint32(math.Max(1, math.Floor(h*scale)))
}
func (r *Renderer) resizeServo() {
	if r.servo == nil {
		return
	}
	width, height := r.servoViewport()
	if width != r.servo.width || height != r.servo.height {
		r.servo.width, r.servo.height = width, height
		r.servoSend(engine.Command{Op: "resize", Width: width, Height: height})
	}
}
func (r *Renderer) renderServo() {
	view := r.servo
	if view == nil {
		return
	}
	select {
	case err := <-view.session.Errors:
		r.servoError = err
	default:
	}
	select {
	case state := <-view.session.States:
		if state.LinkCount < 0 || state.LinkCount > 2048 {
			r.servoError = errors.New("invalid Servo link count")
			return
		}
		view.links = state.LinkCount
		r.scrollY = state.ScrollY
		r.docTitle = state.Title
		if state.SelectedLink == r.selectedLink {
			view.selectedURL = state.SelectedURL
			view.tableScrollLeft = state.TableScrollLeft
			view.tableScrollRight = state.TableScrollRight
		}
	default:
	}
	select {
	case frame := <-view.session.Frames:
		width, height := int32(frame.Rect.Dx()), int32(frame.Rect.Dy())
		if uint32(width) != view.width || uint32(height) != view.height {
			break
		}
		if view.texture == nil || width != view.textureWidth || height != view.textureHeight {
			if view.texture != nil {
				view.texture.Destroy()
				view.texture = nil
			}
			texture, err := r.sdlRenderer.CreateTexture(sdl.PIXELFORMAT_ABGR8888, sdl.TEXTUREACCESS_STREAMING, width, height)
			if err != nil {
				r.servoError = err
				return
			}
			view.texture = texture
			view.textureWidth, view.textureHeight = width, height
		}
		if err := view.texture.Update(nil, unsafe.Pointer(&frame.Pix[0]), frame.Stride); err != nil {
			r.servoError = err
			return
		}
		if r.GetStatusOverride() == view.loading {
			r.SetStatusOverride("")
		}
	default:
	}
	if view.texture != nil {
		r.sdlRenderer.Copy(view.texture, nil, &sdl.Rect{W: r.width, H: r.height - r.getStatusBarHeight()})
	}
}
