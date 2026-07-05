package ui

import (
	"log/slog"

	"github.com/veandco/go-sdl2/sdl"
)

const (
	scrollStep      = 40
	axisSensitivity = 16000
)

type InputController struct {
	app        *App
	globalKeys map[sdl.Scancode]func()
	docKeys    map[sdl.Scancode]func()
	treeKeys   map[sdl.Scancode]func()
}

func NewInputController(app *App) *InputController {
	c := &InputController{app: app}
	c.globalKeys = map[sdl.Scancode]func(){
		sdl.SCANCODE_Q:         func() { app.running.Store(false) },
		sdl.SCANCODE_H:         func() { app.goHome() },
		sdl.SCANCODE_M:         func() { _ = app.loader.OpenFile("virtual:menu") },
		sdl.SCANCODE_RETURN2:   func() { app.toggleMode() },
		sdl.SCANCODE_T:         func() { app.toggleMode() },
		sdl.SCANCODE_C:         func() { app.viewer.ToggleTheme() },
		sdl.SCANCODE_EQUALS:    func() { _ = app.viewer.Zoom(1) },
		sdl.SCANCODE_KP_PLUS:   func() { _ = app.viewer.Zoom(1) },
		sdl.SCANCODE_MINUS:     func() { _ = app.viewer.Zoom(-1) },
		sdl.SCANCODE_KP_MINUS:  func() { _ = app.viewer.Zoom(-1) },
		sdl.SCANCODE_ESCAPE:    func() { app.goBack() },
		sdl.SCANCODE_BACKSPACE: func() { app.goBack() },
	}
	c.docKeys = map[sdl.Scancode]func(){
		sdl.SCANCODE_UP:       func() { app.scroller.ScrollBy(-scrollStep) },
		sdl.SCANCODE_W:        func() { app.scroller.ScrollBy(-scrollStep) },
		sdl.SCANCODE_KP_8:     func() { app.scroller.ScrollBy(-scrollStep) },
		sdl.SCANCODE_DOWN:     func() { app.scroller.ScrollBy(scrollStep) },
		sdl.SCANCODE_S:        func() { app.scroller.ScrollBy(scrollStep) },
		sdl.SCANCODE_KP_2:     func() { app.scroller.ScrollBy(scrollStep) },
		sdl.SCANCODE_LEFT:     func() { app.links.SelectPrevLink() },
		sdl.SCANCODE_A:        func() { app.links.SelectPrevLink() },
		sdl.SCANCODE_KP_4:     func() { app.links.SelectPrevLink() },
		sdl.SCANCODE_RIGHT:    func() { app.links.SelectNextLink() },
		sdl.SCANCODE_D:        func() { app.links.SelectNextLink() },
		sdl.SCANCODE_KP_6:     func() { app.links.SelectNextLink() },
		sdl.SCANCODE_PAGEUP:   func() { app.scroller.ScrollPageUp() },
		sdl.SCANCODE_PAGEDOWN: func() { app.scroller.ScrollPageDown() },
		sdl.SCANCODE_SPACE:    func() { app.scroller.ScrollPageDown() },
		sdl.SCANCODE_RETURN:   func() { c.openSelectedLink() },
		sdl.SCANCODE_KP_ENTER: func() { c.openSelectedLink() },
	}
	c.treeKeys = map[sdl.Scancode]func(){
		sdl.SCANCODE_UP:       func() { app.navState.MoveUp() },
		sdl.SCANCODE_W:        func() { app.navState.MoveUp() },
		sdl.SCANCODE_KP_8:     func() { app.navState.MoveUp() },
		sdl.SCANCODE_DOWN:     func() { app.navState.MoveDown() },
		sdl.SCANCODE_S:        func() { app.navState.MoveDown() },
		sdl.SCANCODE_KP_2:     func() { app.navState.MoveDown() },
		sdl.SCANCODE_RIGHT:    func() { c.treeActionRight() },
		sdl.SCANCODE_D:        func() { c.treeActionRight() },
		sdl.SCANCODE_KP_6:     func() { c.treeActionRight() },
		sdl.SCANCODE_LEFT:     func() { app.navState.ActionLeft() },
		sdl.SCANCODE_A:        func() { app.navState.ActionLeft() },
		sdl.SCANCODE_KP_4:     func() { app.navState.ActionLeft() },
		sdl.SCANCODE_RETURN:   func() { c.handleTreeSelection() },
		sdl.SCANCODE_KP_ENTER: func() { c.handleTreeSelection() },
	}
	return c
}

func (c *InputController) openSelectedLink() {
	url := c.app.links.SelectedLinkURL()
	if url != "" {
		c.app.loader.NavigateLink(url)
	}
}

func (c *InputController) treeActionRight() {
	if c.app.navState.CursorExpandable() {
		c.app.navState.ActionRight()
	} else {
		c.handleTreeSelection()
	}
}

func (c *InputController) ProcessEvent(event sdl.Event) {
	app := c.app
	switch e := event.(type) {
	case *sdl.QuitEvent:
		app.running.Store(false)

	case *sdl.KeyboardEvent:
		if e.Type != sdl.KEYDOWN {
			return
		}
		sc := e.Keysym.Scancode
		debugEvent("KEY", int(sc), 0)

		if fn, ok := c.globalKeys[sc]; ok {
			fn()
			if sc != sdl.SCANCODE_ESCAPE && sc != sdl.SCANCODE_BACKSPACE {
				return
			}
		}

		if app.mode == modeTree {
			if fn, ok := c.treeKeys[sc]; ok {
				fn()
			}
		} else {
			if fn, ok := c.docKeys[sc]; ok {
				fn()
			}
		}
		if app.mode == modeTree {
			app.renderTree()
		}

	case *sdl.ControllerAxisEvent, *sdl.ControllerButtonEvent:
		if action, ok := app.gamepad.TranslateEvent(event, app.mode); ok {
			if action != ActionNone {
				var val int16
				if ax, ok := event.(*sdl.ControllerAxisEvent); ok {
					val = ax.Value
				}
				c.executeGamepadAction(action, val)
			}
		}

	case *sdl.WindowEvent:
		if e.Event == sdl.WINDOWEVENT_RESIZED ||
			e.Event == sdl.WINDOWEVENT_SIZE_CHANGED {
			app.viewer.Relayout()
		}
		if e.Event == sdl.WINDOWEVENT_LEAVE {
			app.viewer.HandleMouseLeave()
		}

	case *sdl.MouseMotionEvent:
		app.viewer.HandleMouseMove(e.X, e.Y)

	case *sdl.TouchFingerEvent:
		app.viewer.HandleTouch()

	case *sdl.MouseWheelEvent:
		app.scroller.ScrollBy(-scrollStep * e.Y)

	case *sdl.MouseButtonEvent:
		if e.Type == sdl.MOUSEBUTTONDOWN {
			switch e.Button {
			case sdl.BUTTON_LEFT:
				switch app.mode {
				case modeDoc:
					url := app.links.HandleClick(e.X, e.Y)
					if url != "" {
						app.loader.NavigateLink(url)
					}
				case modeTree:
					idx := app.scroller.HandleTreeClick(e.X, e.Y)
					if idx >= 0 {
						app.navState.MoveTo(idx)
						c.handleTreeSelection()
					}
				}
			case sdl.BUTTON_RIGHT, sdl.BUTTON_X1:
				c.processJoyB() // processJoyB does exactly what "Back" does in both modes!
			}
		}
	}
}

func (c *InputController) processJoyB() {
	c.app.goBack()
}

func (c *InputController) executeGamepadAction(action Action, val int16) {
	app := c.app
	switch action {
	case ActionOpenEnter:
		if app.mode == modeTree {
			c.handleTreeSelection()
		} else {
			c.openSelectedLink()
		}
	case ActionBack:
		c.processJoyB()
	case ActionScrollUp:
		if app.mode == modeTree {
			app.navState.MoveUp()
			app.renderTree()
		} else {
			if val != 0 {
				app.scroller.ScrollBy(-scrollStep * int32(-val/axisSensitivity))
			} else {
				app.scroller.ScrollBy(-scrollStep)
			}
		}
	case ActionScrollDown:
		if app.mode == modeTree {
			app.navState.MoveDown()
			app.renderTree()
		} else {
			if val != 0 {
				app.scroller.ScrollBy(scrollStep * int32(val/axisSensitivity))
			} else {
				app.scroller.ScrollBy(scrollStep)
			}
		}
	case ActionPageUp:
		app.scroller.ScrollPageUp()
	case ActionPageDown:
		app.scroller.ScrollPageDown()
	case ActionToggleTree:
		app.toggleMode()
	case ActionGoHome:
		app.goHome()
	case ActionQuit:
		app.running.Store(false)
	case ActionZoomIn:
		_ = app.viewer.Zoom(1)
	case ActionZoomOut:
		_ = app.viewer.Zoom(-1)
	case ActionSelectPrevLink:
		app.links.SelectPrevLink()
	case ActionSelectNextLink:
		app.links.SelectNextLink()
	case ActionToggleTheme:
		app.viewer.ToggleTheme()
	}
}

func debugEvent(kind string, code int, val int) {
	slog.Debug("Input event", "kind", kind, "code", code, "val", val)
}

func (c *InputController) handleTreeSelection() {
	app := c.app
	if app.navState.CursorExpandable() {
		app.navState.ActionRight()
	} else if app.navState.CursorIsLeaf() {
		path := app.navState.CursorPath()
		if path != "" {
			app.navigator.Open("virtual:tree")
			app.loader.NavigateLink(path)
		}
	}
	if app.mode == modeTree {
		app.renderTree()
	}
}
