package ui

import (
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/veandco/go-sdl2/sdl"
)

const (
	scrollStep     = 40
	holdDelay      = 200 * time.Millisecond
	maxAxisValue   = 32767
	analogMaxSpeed = float64(scrollStep * 5)
	maxHoldSpeedup = 16.0
	maxHoldElapsed = 100 * time.Millisecond
)

type scrollHold struct {
	active    bool
	started   time.Time
	lastStep  time.Time
	direction int32
}

type analogScrollHold struct {
	scrollHold
	remainder float64
}

// delta integrates pixels over time, including subpixel motion at low deflection.
// A held stick keeps scrolling without additional SDL axis events.
func (h *analogScrollHold) delta(value int16, now time.Time) int32 {
	direction, strength := analogDirection(value)
	if direction == 0 {
		*h = analogScrollHold{}
		return 0
	}
	if !h.active || h.direction != direction {
		*h = analogScrollHold{scrollHold: scrollHold{active: true, started: now, lastStep: now, direction: direction}}
		return 0
	}
	elapsed := max(time.Duration(0), min(now.Sub(h.lastStep), maxHoldElapsed))
	h.lastStep = now
	h.remainder += analogMaxSpeed * strength * holdSpeedup(now.Sub(h.started)-holdDelay) * elapsed.Seconds()
	pixels := int32(h.remainder)
	h.remainder -= float64(pixels)
	return direction * pixels
}

type InputController struct {
	app        *App
	globalKeys map[sdl.Scancode]func()
	docKeys    map[sdl.Scancode]func()
	treeKeys   map[sdl.Scancode]func()
	treeHold   scrollHold
	axisHold   analogScrollHold
	linkHold   scrollHold
	tableHold  analogScrollHold
}

func NewInputController(app *App) *InputController {
	c := &InputController{app: app}
	c.globalKeys = map[sdl.Scancode]func(){
		sdl.SCANCODE_Q:         func() { app.running.Store(false) },
		sdl.SCANCODE_H:         func() { app.goHome() },
		sdl.SCANCODE_M:         func() { _ = app.loader.OpenFile("virtual:menu") },
		sdl.SCANCODE_F1:        func() { _ = app.loader.OpenFile("virtual:help") },
		sdl.SCANCODE_F2:        func() { _ = app.loader.OpenFile("virtual:settings") },
		sdl.SCANCODE_RETURN2:   func() { app.toggleMode() },
		sdl.SCANCODE_T:         func() { app.toggleMode() },
		sdl.SCANCODE_EQUALS:    func() { c.zoom(1) },
		sdl.SCANCODE_KP_PLUS:   func() { c.zoom(1) },
		sdl.SCANCODE_MINUS:     func() { c.zoom(-1) },
		sdl.SCANCODE_KP_MINUS:  func() { c.zoom(-1) },
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
		sdl.SCANCODE_LEFT:     func() { c.moveLinkSelection(-1) },
		sdl.SCANCODE_A:        func() { c.moveLinkSelection(-1) },
		sdl.SCANCODE_KP_4:     func() { c.moveLinkSelection(-1) },
		sdl.SCANCODE_RIGHT:    func() { c.moveLinkSelection(1) },
		sdl.SCANCODE_D:        func() { c.moveLinkSelection(1) },
		sdl.SCANCODE_KP_6:     func() { c.moveLinkSelection(1) },
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
		c.markLinkVisited(url)
		c.app.loader.NavigateLink(url)
	}
}

func (c *InputController) moveLinkSelection(direction int32) {
	c.app.loader.cancelPendingDelete()
	if direction < 0 {
		c.app.links.SelectPrevLink()
		return
	}
	c.app.links.SelectNextLink()
}

func (c *InputController) markLinkVisited(url string) {
	if strings.HasPrefix(c.app.navigator.Current(), "virtual:") {
		return
	}
	c.app.viewer.MarkLinkVisited(url)
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
		if e.Repeat != 0 && (sc == sdl.SCANCODE_RETURN || sc == sdl.SCANCODE_KP_ENTER) {
			return
		}
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

	case *sdl.ControllerAxisEvent, *sdl.ControllerButtonEvent, *sdl.ControllerDeviceEvent:
		if action, ok := app.gamepad.TranslateEvent(event, app.mode); ok {
			if action != ActionNone {
				c.executeGamepadAction(action)
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
						c.markLinkVisited(url)
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

// Update advances held D-pad navigation and analog scrolling once per frame.
func (c *InputController) Update(now time.Time) {
	controller := c.app.gamepad.Controller()
	upHeld := controller != nil && controller.Button(sdl.CONTROLLER_BUTTON_DPAD_UP) != 0
	downHeld := controller != nil && controller.Button(sdl.CONTROLLER_BUTTON_DPAD_DOWN) != 0
	leftHeld := controller != nil && controller.Button(sdl.CONTROLLER_BUTTON_DPAD_LEFT) != 0
	rightHeld := controller != nil && controller.Button(sdl.CONTROLLER_BUTTON_DPAD_RIGHT) != 0
	axisDirection, axisStrength := analogDirection(c.app.gamepad.LeftY())

	if c.app.mode == modeTree {
		direction := int32(0)
		strength := float64(0)
		if upHeld {
			direction, strength = -1, 1
		} else if downHeld {
			direction, strength = 1, 1
		} else if axisDirection != 0 {
			direction, strength = axisDirection, axisStrength
		}
		c.updateTreeHold(direction, strength, now)
		c.axisHold = analogScrollHold{}
		c.linkHold.active = false
		c.tableHold = analogScrollHold{}
		return
	}
	c.treeHold.active = false

	linkDirection := int32(0)
	switch {
	case upHeld || leftHeld:
		linkDirection = -1
	case downHeld || rightHeld:
		linkDirection = 1
	}
	c.updateLinkHold(linkDirection, now)

	if delta := c.tableHold.delta(c.app.gamepad.LeftX(), now); delta != 0 {
		if scroller, ok := c.app.viewer.(TableScroller); ok {
			scroller.ScrollTableHorizontally(delta)
		}
	}
	if delta := c.axisHold.delta(c.app.gamepad.LeftY(), now); delta != 0 {
		c.app.scroller.ScrollBy(delta)
	}
}

func (c *InputController) updateLinkHold(direction int32, now time.Time) {
	if direction == 0 {
		c.linkHold.active = false
		return
	}
	if !c.linkHold.active || c.linkHold.direction != direction {
		c.linkHold = scrollHold{active: true, started: now, lastStep: now, direction: direction}
		return
	}
	if now.Sub(c.linkHold.started) < holdDelay {
		return
	}
	interval := 200 * time.Millisecond
	if now.Sub(c.linkHold.started) > time.Second {
		interval = 120 * time.Millisecond
	}
	if now.Sub(c.linkHold.lastStep) < interval {
		return
	}
	c.moveLinkSelection(direction)
	c.linkHold.lastStep = now
}

func (c *InputController) updateTreeHold(direction int32, strength float64, now time.Time) {
	h := &c.treeHold
	if direction == 0 {
		h.active = false
		return
	}
	if !h.active || h.direction != direction {
		*h = scrollHold{active: true, started: now, lastStep: now, direction: direction}
		return
	}
	if now.Sub(h.started) < holdDelay {
		return
	}
	rate := min(40, 5*holdSpeedup(now.Sub(h.started))*strength)
	if rate <= 0 {
		return
	}
	interval := time.Duration(float64(time.Second) / rate)
	if now.Sub(h.lastStep) < interval {
		return
	}
	steps := int(min(now.Sub(h.lastStep), maxHoldElapsed) / interval)
	// Low deflection can have an interval longer than the catch-up budget.
	steps = max(1, steps)
	for i := 0; i < steps; i++ {
		if direction < 0 {
			c.app.navState.MoveUp()
		} else {
			c.app.navState.MoveDown()
		}
	}
	c.app.renderTree()
	h.lastStep = now
}

// Clamp before exponentiation so even a long hold cannot overflow the scroll
// delta or produce a zero repeat interval. Limit catch-up separately after stalls.
func holdSpeedup(held time.Duration) float64 {
	return math.Pow(2, min(math.Log2(maxHoldSpeedup), max(0, held.Seconds())))
}

func analogDirection(value int16) (int32, float64) {
	absolute := math.Abs(float64(value))
	if absolute <= float64(analogDeadZone) {
		return 0, 0
	}
	strength := (absolute - float64(analogDeadZone)) / (maxAxisValue - float64(analogDeadZone))
	if strength > 1 {
		strength = 1
	}
	if value < 0 {
		return -1, strength
	}
	return 1, strength
}

func (c *InputController) processJoyB() {
	c.app.goBack()
}

func (c *InputController) executeGamepadAction(action Action) {
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
	case ActionLeft:
		if app.mode == modeTree {
			app.navState.ActionLeft()
			app.renderTree()
		}
	case ActionScrollUp:
		if app.mode == modeTree {
			app.navState.MoveUp()
			app.renderTree()
		} else {
			app.scroller.ScrollBy(-scrollStep)
		}
	case ActionScrollDown:
		if app.mode == modeTree {
			app.navState.MoveDown()
			app.renderTree()
		} else {
			app.scroller.ScrollBy(scrollStep)
		}
	case ActionPageUp:
		app.scroller.ScrollPageUp()
	case ActionPageDown:
		app.scroller.ScrollPageDown()
	case ActionToggleTree:
		app.toggleMode()
	case ActionGoHome:
		app.goHome()
	case ActionZoomIn:
		c.zoom(1)
	case ActionZoomOut:
		c.zoom(-1)
	case ActionSelectPrevLink:
		c.moveLinkSelection(-1)
	case ActionSelectNextLink:
		c.moveLinkSelection(1)
	case ActionShowHelp:
		_ = app.loader.OpenFile("virtual:help")
	case ActionShowSettings:
		_ = app.loader.OpenFile("virtual:settings")
	}
}

func (c *InputController) zoom(delta int) {
	if err := c.app.viewer.Zoom(delta); err != nil {
		slog.Error("Zoom failed", "delta", delta, "error", err)
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
