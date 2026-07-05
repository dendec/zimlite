package ui

import "github.com/veandco/go-sdl2/sdl"

const (
	triggerPressThreshold   int16 = 20000
	triggerReleaseThreshold int16 = 10000
	analogDeadZone          int16 = 8000
)

// Action represents a high-level application action triggered by the gamepad.
type Action int

const (
	ActionNone Action = iota
	ActionOpenEnter
	ActionBack
	ActionLeft
	ActionScrollUp
	ActionScrollDown
	ActionPageUp
	ActionPageDown
	ActionToggleTree
	ActionGoHome
	ActionQuit
	ActionZoomIn
	ActionZoomOut
	ActionSelectPrevLink
	ActionSelectNextLink
	ActionShowHelp
	ActionShowSettings
)

// TriggerDebouncer translates continuous analog axis values into discrete digital press events.
type TriggerDebouncer struct {
	pressed bool
}

// Update processes a new raw axis value. Returns true if this update represents a new "press" transition.
func (td *TriggerDebouncer) Update(value int16) bool {
	if value > triggerPressThreshold && !td.pressed {
		td.pressed = true
		return true
	} else if value < triggerReleaseThreshold {
		td.pressed = false
	}
	return false
}

// GamepadState groups all tracked inputs for a controller and translates raw events to logical Actions.
type GamepadState struct {
	L2            TriggerDebouncer
	R2            TriggerDebouncer
	selectPressed bool
	menuPressed   bool
	startPressed  bool
}

// TranslateEvent processes a raw SDL event. If the event is a GameController event, it translates it to an Action and returns true.
func (g *GamepadState) TranslateEvent(event sdl.Event, mode appMode) (Action, bool) {
	switch e := event.(type) {
	case *sdl.ControllerAxisEvent:
		v := e.Value
		// Handle triggers
		switch e.Axis {
		case sdl.CONTROLLER_AXIS_TRIGGERLEFT:
			if g.L2.Update(v) {
				return ActionZoomOut, true
			}
			return ActionNone, true
		case sdl.CONTROLLER_AXIS_TRIGGERRIGHT:
			if g.R2.Update(v) {
				return ActionZoomIn, true
			}
			return ActionNone, true
		}

		// Dead zone
		if v > -analogDeadZone && v < analogDeadZone {
			return ActionNone, false
		}

		if mode == modeTree {
			switch e.Axis {
			case sdl.CONTROLLER_AXIS_LEFTY: // vertical
				if v < 0 {
					return ActionScrollUp, true
				}
				return ActionScrollDown, true
			case sdl.CONTROLLER_AXIS_LEFTX: // horizontal
				if v < 0 {
					return ActionLeft, true // collapses branch / moves to parent
				}
				return ActionOpenEnter, true // translates to ActionRight
			}
		} else {
			switch e.Axis {
			case sdl.CONTROLLER_AXIS_LEFTY:
				if v < 0 {
					return ActionScrollUp, true
				}
				return ActionScrollDown, true
			case sdl.CONTROLLER_AXIS_LEFTX:
				if v < 0 {
					return ActionSelectPrevLink, true
				}
				return ActionSelectNextLink, true
			}
		}

	case *sdl.ControllerButtonEvent:
		switch e.Button {
		case sdl.CONTROLLER_BUTTON_BACK: // Select
			if e.State == sdl.RELEASED {
				g.selectPressed = false
				return ActionNone, true
			}
			g.selectPressed = true
			if g.menuPressed {
				return ActionQuit, true
			}
			return ActionNone, true
		case sdl.CONTROLLER_BUTTON_GUIDE: // Menu
			if e.State == sdl.RELEASED {
				g.menuPressed = false
				return ActionNone, true
			}
			g.menuPressed = true
			if g.selectPressed {
				return ActionQuit, true
			}
			return ActionShowSettings, true
		case sdl.CONTROLLER_BUTTON_START:
			if e.State == sdl.RELEASED {
				g.startPressed = false
				return ActionNone, true
			}
			g.startPressed = true
			return ActionShowHelp, true
		default:
			if e.State != sdl.PRESSED {
				return ActionNone, true
			}
			switch e.Button {
			case sdl.CONTROLLER_BUTTON_A:
				return ActionOpenEnter, true
			case sdl.CONTROLLER_BUTTON_B:
				return ActionBack, true
			case sdl.CONTROLLER_BUTTON_X:
				return ActionToggleTree, true
			case sdl.CONTROLLER_BUTTON_Y:
				return ActionGoHome, true
			case sdl.CONTROLLER_BUTTON_LEFTSHOULDER:
				return ActionPageUp, true
			case sdl.CONTROLLER_BUTTON_RIGHTSHOULDER:
				return ActionPageDown, true
			case sdl.CONTROLLER_BUTTON_DPAD_UP:
				return ActionScrollUp, true
			case sdl.CONTROLLER_BUTTON_DPAD_DOWN:
				return ActionScrollDown, true
			case sdl.CONTROLLER_BUTTON_DPAD_LEFT:
				if mode == modeTree {
					return ActionLeft, true
				}
				return ActionSelectPrevLink, true
			case sdl.CONTROLLER_BUTTON_DPAD_RIGHT:
				if mode == modeTree {
					return ActionOpenEnter, true
				}
				return ActionSelectNextLink, true
			}
		}
	}

	return ActionNone, false
}
