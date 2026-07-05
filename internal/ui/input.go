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
	app *App
}

func NewInputController(app *App) *InputController {
	return &InputController{app: app}
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

		// Global keys (work in both modes).
		switch sc {
		case sdl.SCANCODE_Q:
			app.running.Store(false)
			return
		case sdl.SCANCODE_H: // H = go home
			app.goHome()
			return
		case sdl.SCANCODE_M: // M = open file menu
			_ = app.loader.OpenFile("virtual:menu")
			return
		case sdl.SCANCODE_RETURN2, sdl.SCANCODE_T: // T = toggle tree mode
			app.toggleMode()
			return
		case sdl.SCANCODE_C: // C = toggle dark/light theme
			app.viewer.ToggleTheme()
			return
		case sdl.SCANCODE_EQUALS, sdl.SCANCODE_KP_PLUS: // + = zoom in
			_ = app.viewer.Zoom(1)
			return
		case sdl.SCANCODE_MINUS, sdl.SCANCODE_KP_MINUS: // - = zoom out
			_ = app.viewer.Zoom(-1)
			return
		case sdl.SCANCODE_ESCAPE, sdl.SCANCODE_BACKSPACE:
			// Global back — also works as doc back.
		}

		// Mode-specific handling.
		if app.mode == modeTree {
			c.processTreeKey(sc)
		} else {
			c.processDocKey(sc)
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

	case *sdl.MouseMotionEvent:
		app.viewer.HandleMouseMove(e.X, e.Y)

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

func (c *InputController) processTreeKey(sc sdl.Scancode) {
	app := c.app
	switch sc {
	case sdl.SCANCODE_UP, sdl.SCANCODE_W, sdl.SCANCODE_KP_8:
		app.navState.MoveUp()
	case sdl.SCANCODE_DOWN, sdl.SCANCODE_S, sdl.SCANCODE_KP_2:
		app.navState.MoveDown()
	case sdl.SCANCODE_RIGHT, sdl.SCANCODE_D, sdl.SCANCODE_KP_6:
		if app.navState.CursorExpandable() {
			app.navState.ActionRight()
		} else {
			c.handleTreeSelection()
		}
	case sdl.SCANCODE_LEFT, sdl.SCANCODE_A, sdl.SCANCODE_KP_4:
		app.navState.ActionLeft()
	case sdl.SCANCODE_RETURN, sdl.SCANCODE_KP_ENTER:
		if app.navState.Cursor != nil {
			slog.Debug("Tree navigation enter", "label", app.navState.Cursor.Label(), "isLeaf", app.navState.CursorIsLeaf(), "path", app.navState.CursorPath())
		}
		c.handleTreeSelection()
	case sdl.SCANCODE_ESCAPE, sdl.SCANCODE_BACKSPACE:
		app.goBack()
	}
	if app.mode == modeTree {
		app.renderTree()
	}
}

func (c *InputController) processDocKey(sc sdl.Scancode) {
	app := c.app
	switch sc {
	case sdl.SCANCODE_UP, sdl.SCANCODE_W, sdl.SCANCODE_KP_8:
		app.scroller.ScrollBy(-scrollStep)
	case sdl.SCANCODE_DOWN, sdl.SCANCODE_S, sdl.SCANCODE_KP_2:
		app.scroller.ScrollBy(scrollStep)
	case sdl.SCANCODE_LEFT, sdl.SCANCODE_A, sdl.SCANCODE_KP_4:
		app.links.SelectPrevLink()
	case sdl.SCANCODE_RIGHT, sdl.SCANCODE_D, sdl.SCANCODE_KP_6:
		app.links.SelectNextLink()
	case sdl.SCANCODE_PAGEUP:
		app.scroller.ScrollPageUp()
	case sdl.SCANCODE_PAGEDOWN, sdl.SCANCODE_SPACE:
		app.scroller.ScrollPageDown()
	case sdl.SCANCODE_RETURN, sdl.SCANCODE_KP_ENTER:
		url := app.links.SelectedLinkURL()
		if url != "" {
			app.loader.NavigateLink(url)
		}
	case sdl.SCANCODE_ESCAPE, sdl.SCANCODE_BACKSPACE:
		app.goBack()
	}
}

func (c *InputController) processJoyA() {
	app := c.app
	if app.mode == modeTree {
		c.handleTreeSelection()
	} else {
		url := app.links.SelectedLinkURL()
		if url != "" {
			app.loader.NavigateLink(url)
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
		c.processJoyA()
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
