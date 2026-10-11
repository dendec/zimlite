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

// AxisDebouncer debounces dual-direction analog axes (thumbsticks, D-pad-as-axis).
// Fires once on transition to an active direction, ignores subsequent
// same-direction values until released or reversed.
type AxisDebouncer struct {
	direction int16
}

// Update fires on a transition from neutral or the opposite direction.
func (ad *AxisDebouncer) Update(value int16, threshold int16) bool {
	direction := int16(0)
	if value < -threshold {
		direction = -1
	} else if value > threshold {
		direction = 1
	}
	changed := direction != 0 && direction != ad.direction
	ad.direction = direction
	return changed
}

// GamepadState groups all tracked inputs for a controller and translates raw events to logical Actions.
type GamepadState struct {
	L2         TriggerDebouncer
	R2         TriggerDebouncer
	leftY      AxisDebouncer
	leftX      AxisDebouncer
	controller *sdl.GameController
	leftYValue int16
	leftXValue int16
}

func (g *GamepadState) SetController(controller *sdl.GameController) {
	g.controller = controller
}

func (g *GamepadState) Controller() *sdl.GameController {
	return g.controller
}

func (g *GamepadState) LeftY() int16 {
	if g.controller != nil {
		return g.controller.Axis(sdl.CONTROLLER_AXIS_LEFTY)
	}
	return g.leftYValue
}

func (g *GamepadState) LeftX() int16 {
	if g.controller != nil {
		return g.controller.Axis(sdl.CONTROLLER_AXIS_LEFTX)
	}
	return g.leftXValue
}

// TranslateEvent processes a raw SDL event. If the event is a GameController event, it translates it to an Action and returns true.
func (g *GamepadState) TranslateEvent(event sdl.Event, mode appMode) (Action, bool) {
	switch e := event.(type) {
	case *sdl.ControllerAxisEvent:
		v := e.Value
		switch e.Axis {
		case sdl.CONTROLLER_AXIS_LEFTY:
			g.leftYValue = v
		case sdl.CONTROLLER_AXIS_LEFTX:
			g.leftXValue = v
		}
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

		if mode == modeTree {
			switch e.Axis {
			case sdl.CONTROLLER_AXIS_LEFTY: // vertical
				if !g.leftY.Update(v, analogDeadZone) {
					return ActionNone, true
				}
				if v < 0 {
					return ActionScrollUp, true
				}
				return ActionScrollDown, true
			case sdl.CONTROLLER_AXIS_LEFTX: // horizontal
				if !g.leftX.Update(v, analogDeadZone) {
					return ActionNone, true
				}
				if v < 0 {
					return ActionLeft, true // collapses branch / moves to parent
				}
				return ActionOpenEnter, true // translates to ActionRight
			}
		} else {
			switch e.Axis {
			case sdl.CONTROLLER_AXIS_LEFTY:
				if !g.leftY.Update(v, analogDeadZone) {
					return ActionNone, true
				}
				if v < 0 {
					return ActionScrollUp, true
				}
				return ActionScrollDown, true
			case sdl.CONTROLLER_AXIS_LEFTX:
				// Horizontal stick motion scrolls an overflowing table in Update.
				return ActionNone, true
			}
		}

	case *sdl.ControllerButtonEvent:
		switch e.Button {
		case sdl.CONTROLLER_BUTTON_BACK: // Select
			if e.State == sdl.RELEASED {
				return ActionNone, true
			}
			return ActionShowSettings, true
		case sdl.CONTROLLER_BUTTON_GUIDE: // Menu (not on all devices)
			if e.State == sdl.RELEASED {
				return ActionNone, true
			}
			return ActionShowSettings, true
		case sdl.CONTROLLER_BUTTON_START:
			if e.State == sdl.RELEASED {
				return ActionNone, true
			}
			return ActionShowHelp, true
		default:
			if e.State != sdl.PRESSED {
				return ActionNone, true
			}
			switch e.Button {
			case sdl.CONTROLLER_BUTTON_A:
				return ActionBack, true
			case sdl.CONTROLLER_BUTTON_B:
				return ActionOpenEnter, true
			case sdl.CONTROLLER_BUTTON_X:
				return ActionToggleTree, true
			case sdl.CONTROLLER_BUTTON_Y:
				return ActionGoHome, true
			case sdl.CONTROLLER_BUTTON_LEFTSHOULDER:
				return ActionPageUp, true
			case sdl.CONTROLLER_BUTTON_RIGHTSHOULDER:
				return ActionPageDown, true
			case sdl.CONTROLLER_BUTTON_DPAD_UP:
				if mode == modeTree {
					return ActionScrollUp, true
				}
				return ActionSelectPrevLink, true
			case sdl.CONTROLLER_BUTTON_DPAD_DOWN:
				if mode == modeTree {
					return ActionScrollDown, true
				}
				return ActionSelectNextLink, true
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
