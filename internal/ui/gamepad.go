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

// TranslateEvent processes a raw SDL event. If the event is a gamepad event, it translates it to an Action and returns true.
func (g *GamepadState) TranslateEvent(event sdl.Event, mode appMode) (Action, bool) {
	switch e := event.(type) {
	case *sdl.JoyAxisEvent:
		v := e.Value
		// Handle triggers
		switch e.Axis {
		case 2, 4: // L2
			if g.L2.Update(v) {
				return ActionZoomOut, true
			}
			return ActionNone, true
		case 3, 5: // R2
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
			case 1: // vertical
				if v < 0 {
					return ActionScrollUp, true
				}
				return ActionScrollDown, true
			case 0: // horizontal
				if v < 0 {
					return ActionBack, true // translates to ActionLeft
				}
				return ActionOpenEnter, true // translates to ActionRight
			}
		} else {
			switch e.Axis {
			case 1:
				if v < 0 {
					return ActionScrollUp, true
				}
				return ActionScrollDown, true
			case 0:
				if v < 0 {
					return ActionSelectPrevLink, true
				}
				return ActionSelectNextLink, true
			}
		}

	case *sdl.JoyButtonEvent:
		if e.Type != sdl.JOYBUTTONDOWN {
			return ActionNone, true // Swallow release events
		}
		switch e.Button {
		case 0, 1: // A/B
			return ActionOpenEnter, true
		case 2, 3: // X/Y
			return ActionBack, true
		case 4: // L1
			return ActionPageUp, true
		case 5: // R1
			return ActionPageDown, true
		case 6: // Select
			return ActionToggleTree, true
		case 7: // Start
			return ActionGoHome, true
		case 8: // Menu/Guide
			return ActionQuit, true
		case 9: // R2 (button fallback)
			return ActionZoomIn, true
		case 10: // L2 (button fallback)
			return ActionZoomOut, true
		}

	case *sdl.JoyHatEvent:
		switch e.Value {
		case sdl.HAT_UP:
			return ActionScrollUp, true
		case sdl.HAT_DOWN:
			return ActionScrollDown, true
		case sdl.HAT_LEFT:
			if mode == modeTree {
				return ActionBack, true
			}
			return ActionSelectPrevLink, true
		case sdl.HAT_RIGHT:
			if mode == modeTree {
				return ActionOpenEnter, true
			}
			return ActionSelectNextLink, true
		}
	}

	return ActionNone, false
}
