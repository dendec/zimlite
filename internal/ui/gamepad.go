package ui

import "github.com/veandco/go-sdl2/sdl"

// Action represents a high-level application action triggered by the gamepad.
type Action int

const (
	ActionNone Action = iota
	ActionOpenEnter
	ActionBack
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
	ActionToggleTheme
)

// TriggerDebouncer translates continuous analog axis values into discrete digital press events.
type TriggerDebouncer struct {
	pressed bool
}

// Update processes a new raw axis value. Returns true if this update represents a new "press" transition.
func (td *TriggerDebouncer) Update(value int16) bool {
	if value > 20000 && !td.pressed {
		td.pressed = true
		return true // New press transition occurred
	} else if value < 10000 {
		td.pressed = false
	}
	return false
}

// GamepadState groups all tracked inputs for a controller and translates raw events to logical Actions.
type GamepadState struct {
	L2 TriggerDebouncer
	R2 TriggerDebouncer
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
		if v > -8000 && v < 8000 {
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
					return ActionBack, true // translates to ActionLeft
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
		if e.State != sdl.PRESSED {
			return ActionNone, true // Swallow release events
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
		case sdl.CONTROLLER_BUTTON_BACK: // Select
			return ActionToggleTheme, true
		case sdl.CONTROLLER_BUTTON_START:
			return ActionQuit, true
		case sdl.CONTROLLER_BUTTON_DPAD_UP:
			return ActionScrollUp, true
		case sdl.CONTROLLER_BUTTON_DPAD_DOWN:
			return ActionScrollDown, true
		case sdl.CONTROLLER_BUTTON_DPAD_LEFT:
			if mode == modeTree {
				return ActionBack, true
			}
			return ActionSelectPrevLink, true
		case sdl.CONTROLLER_BUTTON_DPAD_RIGHT:
			if mode == modeTree {
				return ActionOpenEnter, true
			}
			return ActionSelectNextLink, true
		}
	}

	return ActionNone, false
}
