package ui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/dendec/zimlite/internal/config"
	"github.com/dendec/zimlite/internal/document"
	"github.com/dendec/zimlite/internal/i18n"
	"github.com/dendec/zimlite/internal/keyboard"
	"github.com/dendec/zimlite/internal/textinput"
	"github.com/veandco/go-sdl2/sdl"
)

type keyboardViewer interface {
	SetKeyboard(*textinput.View)
	KeyboardGeometry() textinput.Geometry
}
type searchState struct {
	systemPreedit                bool
	editing, forcedKeyboard      bool
	app                          *App
	previous                     document.ViewState
	previousMode                 appMode
	archiveID                    string
	languages                    []string
	language                     int
	methodIndex                  int
	profile                      textinput.Profile
	session                      *textinput.Session
	text, preedit, status        string
	observedQuery, issuedQuery   string
	searchDue                    time.Time
	result                       *textinput.Results
	stickDirection               int
	stickRepeat                  time.Time
	labels, candidates           []string
	candidate, focus, layer      int
	visible, busy, pendingSubmit bool
	pendingPicker                string
	pendingLanguage              int
	pendingSequence              int
	picker                       string
	pickerPage                   int
	choices                      []string
	cancel                       context.CancelFunc
	reply                        chan textinput.Results
	offset                       int
	more                         bool
	entries                      []textinput.Result
	lastDirection                int
	heldAt, lastRepeat           time.Time
}

func (app *App) openSearch(forceKeyboard bool) {
	if app.search != nil {
		app.search.focusInput(forceKeyboard)
		app.search.refresh()
		return
	}
	if app.loader.zimReader == nil {
		return
	}
	if _, ok := app.viewer.(keyboardViewer); !ok {
		return
	}
	app.saveCurrentState()
	s := &searchState{app: app, previous: app.navigator.CurrentState(), previousMode: app.mode, pendingLanguage: -1}
	var raw string
	if r, ok := app.loader.zimReader.(interface {
		Metadata(string) (string, error)
		ArchiveID() string
	}); ok {
		raw, _ = r.Metadata("Language")
		s.archiveID = r.ArchiveID()
	}
	if s.archiveID == "" {
		s.archiveID = app.loader.zimReader.FilePath()
	}
	if override := app.config.Get().InputLanguages[s.archiveID]; len(override) > 0 {
		raw = strings.Join(override, ",")
	}
	s.languages = textinput.Languages(raw)
	preferred := app.config.Get().InputLanguage[s.archiveID]
	for i, code := range s.languages {
		if code == preferred {
			s.language = i
			break
		}
	}
	if viewer, ok := app.viewer.(ArticleViewer); ok {
		viewer.StopArticle()
	}
	app.mode = modeDoc
	app.search = s
	s.showResults(nil)
	s.focusInput(forceKeyboard)
}

// focusInput decides panel visibility only while a text field owns input.
func (s *searchState) focusInput(forceKeyboard bool) {
	s.editing = true
	s.forcedKeyboard = forceKeyboard
	s.visible = forceKeyboard || !keyboard.Present()
	if s.visible {
		if len(s.profile.Methods) == 0 {
			s.startLanguage()
		} else if s.profile.Methods[s.methodIndex] != "direct" && s.session == nil {
			s.startMethod()
		}
	}
	s.stickDirection = 0
	s.lastDirection = 0
	sdl.StartTextInput()
	if s.result == nil && s.reply == nil {
		s.showResults(nil)
	}
	s.updateHelp()
	s.refresh()
}
func (s *searchState) tr(key string) string { return i18n.T(s.app.config.Get().Language, key) }
func (s *searchState) release() {
	if s.session != nil {
		s.session.Close()
		s.session = nil
	}
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if v, ok := s.app.viewer.(keyboardViewer); ok {
		v.SetKeyboard(nil)
	}
	sdl.StopTextInput()
}
func (app *App) closeSearch(restore bool) {
	s := app.search
	if s == nil {
		return
	}
	s.release()
	if restore {
		if s.previousMode == modeTree {
			app.mode = modeTree
			app.renderTree()
		} else if err := app.restoreView(app.navigator.Current(), s.previous); err != nil {
			app.viewer.SetStatusOverride(err.Error())
		}
	}
	app.search = nil
}
func (s *searchState) persist() {
	code := s.languages[s.language]
	s.app.config.Update(func(c *config.Config) {
		if c.InputLanguage == nil {
			c.InputLanguage = map[string]string{}
		}
		c.InputLanguage[s.archiveID] = code
		if len(s.profile.Methods) > s.methodIndex {
			if c.InputMethods == nil {
				c.InputMethods = map[string]string{}
			}
			c.InputMethods[code] = s.profile.Methods[s.methodIndex]
		}
	})
	if err := s.app.config.Save(); err != nil {
		slog.Warn("Save input preference", "error", err)
	}
}
func (s *searchState) startLanguage() {
	if s.session != nil {
		s.session.Close()
		s.session = nil
	}
	s.preedit = ""
	s.labels = nil
	s.candidates = nil
	s.layer = 0
	s.focus = 0
	s.methodIndex = 0
	s.picker = ""
	code := s.languages[s.language]
	profile, ok := textinput.Lookup(code)
	s.profile = profile
	if !ok {
		s.status = s.tr("keyboard.unavailable")
		s.refresh()
		return
	}
	preferred := s.app.config.Get().InputMethods[code]
	for i, m := range profile.Methods {
		if m == preferred {
			s.methodIndex = i
		}
	}
	s.startMethod()
	s.persist()
}
func (s *searchState) startMethod() {
	if s.session != nil {
		s.session.Close()
		s.session = nil
	}
	s.labels = nil
	s.candidates = nil
	s.preedit = ""
	method := s.profile.Methods[s.methodIndex]
	if method == "direct" {
		s.status = ""
	} else {
		s.status = s.tr("keyboard.loading")
		s.session = textinput.Start(s.profile.Language, method, s.text)
	}
	s.refresh()
}
func (s *searchState) changeLanguage(delta int) {
	if s.pendingSubmit || s.pendingLanguage >= 0 || s.pendingPicker != "" {
		return
	}
	next := (s.language + delta + len(s.languages)) % len(s.languages)
	if s.session != nil {
		if s.session.Send("commit", "") {
			s.pendingLanguage = next
			s.pendingSequence = s.session.Sequence()
			s.busy = true
		}
		return
	}
	s.language = next
	s.startLanguage()
}
func (s *searchState) send(kind, value string) {
	if s.pendingSubmit || s.pendingLanguage >= 0 || s.pendingPicker != "" {
		return
	}
	if s.session != nil {
		if !s.session.Send(kind, value) {
			s.status = s.tr("keyboard.busy")
		}
		return
	}
	if kind == "key" && (len(s.profile.Methods) == 0 || s.profile.Methods[s.methodIndex] != "direct") {
		return
	}
	switch kind {
	case "key", "text":
		s.text = textinput.Append(s.text, value)
	case "backspace":
		s.text = textinput.Backspace(s.text)
	}
	s.refresh()
}
func (s *searchState) refresh() {
	viewer, ok := s.app.viewer.(keyboardViewer)
	if !ok {
		return
	}
	if !s.visible {
		viewer.SetKeyboard(nil)
		return
	}
	method := ""
	if s.methodIndex < len(s.profile.Methods) {
		method = s.profile.Methods[s.methodIndex]
	}
	layout := s.keyboardLayout()
	if s.focus >= len(layout.Keys) {
		s.focus = 0
	}
	viewer.SetKeyboard(&textinput.View{Text: s.text, Preedit: s.preedit, Language: textinput.Name(s.languages[s.language]), Method: method, Status: s.status, Layout: layout, Layer: s.layer, Candidates: append([]string(nil), s.candidates...), Candidate: s.candidate, Focus: s.focus})
}
func (s *searchState) update(now time.Time) {
	if s.session != nil {
		for {
			st, ok := s.session.Poll()
			if !ok {
				break
			}
			if st.Err != nil {
				s.status = st.Err.Error()
				s.session.Close()
				s.session = nil
				s.pendingSubmit = false
				s.pendingPicker = ""
				s.pendingLanguage = -1
				s.busy = false
				break
			}
			s.text = st.Text
			if !s.systemPreedit {
				s.preedit = st.Preedit
			}
			s.candidates = st.Candidates
			s.candidate = st.Candidate
			if len(st.Labels) > 0 {
				s.labels = st.Labels
			}
			s.status = ""
			if s.pendingPicker != "" && st.Sequence >= s.pendingSequence {
				picker := s.pendingPicker
				s.pendingPicker = ""
				s.pickerOpen(picker)
				break
			}
			if s.pendingLanguage >= 0 && st.Sequence >= s.pendingSequence {
				s.language = s.pendingLanguage
				s.pendingLanguage = -1
				s.busy = false
				s.startLanguage()
				break
			}
			if s.pendingSubmit && st.Sequence >= s.pendingSequence {
				s.pendingSubmit = false
				s.focusResults()
				break
			}
		}
	}
	s.updateQuery(now)
	if s.reply != nil {
		select {
		case result := <-s.reply:
			s.reply = nil
			s.busy = false
			if result.Err != nil {
				s.status = result.Err.Error()
			}
			s.entries = result.Entries
			s.more = result.More
			s.result = &result
			s.showResults(&result)
		default:
		}
	}
	if controller := s.app.gamepad.Controller(); controller != nil {
		direction := 0
		for i, b := range []sdl.GameControllerButton{sdl.CONTROLLER_BUTTON_DPAD_LEFT, sdl.CONTROLLER_BUTTON_DPAD_RIGHT, sdl.CONTROLLER_BUTTON_DPAD_UP, sdl.CONTROLLER_BUTTON_DPAD_DOWN} {
			if controller.Button(b) != 0 {
				direction = i + 1
				break
			}
		}
		if direction == 0 {
			s.lastDirection = 0
		} else if direction != s.lastDirection {
			s.lastDirection = direction
			s.heldAt = now
			s.lastRepeat = now
		} else if now.Sub(s.heldAt) > 300*time.Millisecond && now.Sub(s.lastRepeat) > 90*time.Millisecond {
			s.move(direction)
			s.lastRepeat = now
		}
	}
	s.updateStick(now)
	s.refresh()
}
func (s *searchState) move(direction int) {
	if !s.visible {
		if direction == 1 || direction == 3 {
			s.app.links.SelectPrevLink()
		} else {
			s.app.links.SelectNextLink()
		}
		return
	}
	if s.focus < 0 {
		switch direction {
		case 1:
			s.send("key", "Left")
		case 2:
			s.send("key", "Right")
		case 3:
			s.send("key", "Up")
		case 4:
			s.focus = 0
		}
		return
	}
	layout := s.keyboardLayout()
	if s.focus >= 0 && s.focus < len(layout.Keys) && layout.Keys[s.focus].Row == 0 && direction == 3 && len(s.candidates) > 0 {
		s.focus = -1
	} else {
		s.focus = layout.Move(s.focus, direction)
	}
	s.refresh()
}
func (s *searchState) keyboardLayout() textinput.Layout {
	if s.picker == "" {
		return textinput.Keyboard(s.layer, s.labels)
	}
	labels, values := []string{}, []string{}
	for i := s.pickerPage * 9; i < min(len(s.choices), (s.pickerPage+1)*9); i++ {
		value := s.choices[i]
		label := value
		if s.picker != "methods" && value != "+" && value != "@method" {
			label = textinput.Name(value)
		}
		if value == "@method" && s.methodIndex < len(s.profile.Methods) {
			label = "⌨ · " + s.profile.Methods[s.methodIndex]
		}
		labels = append(labels, label)
		values = append(values, value)
	}
	return textinput.Picker(labels, values)
}

func (s *searchState) changeLayer(layer int) {
	previous := s.keyboardLayout()
	focus := s.focus
	if layer == s.layer || focus < 0 || focus >= len(previous.Keys) {
		s.layer = layer
		return
	}
	selected := previous.Keys[focus]
	s.layer = layer
	next := s.keyboardLayout()

	// Keep special controls (especially ?123/ABC) selected by their action,
	// even when the number of keys before them changes between layouts.
	if selected.Action != textinput.Character {
		for i, key := range next.Keys {
			if key.Action == selected.Action && key.Value == selected.Value {
				s.focus = i
				return
			}
		}
		for i, key := range next.Keys {
			if key.Action == selected.Action {
				s.focus = i
				return
			}
		}
	}

	// Character keys are laid out differently for some scripts. Preserve the
	// closest key on the same row instead of reusing a now-unrelated index.
	if previous.Units > 0 && next.Units > 0 {
		oldCenter := 2*selected.Start + selected.Span
		best, bestDistance := -1, int64(1<<62)
		for i, key := range next.Keys {
			if key.Row != selected.Row {
				continue
			}
			center := 2*key.Start + key.Span
			delta := int64(center)*int64(previous.Units) - int64(oldCenter)*int64(next.Units)
			if delta < 0 {
				delta = -delta
			}
			if delta < bestDistance {
				best, bestDistance = i, delta
			}
		}
		if best >= 0 {
			s.focus = best
			return
		}
	}
	if len(next.Keys) == 0 {
		s.focus = 0
	} else {
		s.focus = min(focus, len(next.Keys)-1)
	}
}

func (s *searchState) pickerOpen(kind string) {
	if s.preedit != "" && s.session != nil {
		if s.pendingPicker != "" {
			return
		}
		if s.session.Send("commit", "") {
			s.pendingPicker = kind
			s.pendingSequence = s.session.Sequence()
		}
		return
	}
	if s.picker == kind {
		s.picker = ""
		s.refresh()
		return
	}
	s.picker = kind
	s.pickerPage = 0
	s.choices = nil
	s.focus = 0
	switch kind {
	case "languages":
		s.choices = append(s.choices, s.languages...)
		if len(s.profile.Methods) > 1 {
			s.choices = append(s.choices, "@method")
		}
		s.choices = append(s.choices, "+")
	case "methods":
		s.choices = append(s.choices, s.profile.Methods...)
	case "all":
		s.choices = textinput.AvailableLanguages()
	}
	s.refresh()
}
func (s *searchState) activate() {
	if !s.visible {
		s.openSelected()
		return
	}
	if s.focus < 0 {
		s.send("key", "Return")
		return
	}
	layout := s.keyboardLayout()
	if s.focus >= len(layout.Keys) {
		return
	}
	key := layout.Keys[s.focus]
	switch key.Action {
	case textinput.Pick:
		s.pickChoice(key.Value)
	case textinput.Character, textinput.Literal:
		value := key.Value
		if key.Action == textinput.Literal || s.layer >= 2 && len(s.labels) >= 72 || len([]rune(value)) != 1 || []rune(value)[0] > 127 {
			s.send("text", value)
		} else {
			s.send("key", value)
		}
	case textinput.Shift:
		if s.layer >= 2 {
			s.changeLayer(5 - s.layer)
		} else {
			s.changeLayer(1 - s.layer)
		}
	case textinput.Symbols:
		if s.layer >= 2 {
			s.changeLayer(0)
		} else {
			s.changeLayer(2)
		}
	case textinput.Space:
		s.send("key", " ")
	case textinput.Delete:
		s.send("backspace", "")
	case textinput.Language:
		s.pickerOpen("languages")
	case textinput.PreviousPage, textinput.NextPage:
		delta := 1
		if key.Action == textinput.PreviousPage {
			delta = -1
		}
		s.pagePicker(delta)
	case textinput.ClosePicker:
		s.picker = ""
		s.focus = 0
	case textinput.Submit:
		s.submit()
	}
	s.refresh()
}
func (s *searchState) pagePicker(delta int) {
	pages := max(1, (len(s.choices)+8)/9)
	s.pickerPage = (s.pickerPage + delta + pages) % pages
	s.focus = 0
}
func (s *searchState) pickChoice(choice string) {
	switch s.picker {
	case "languages":
		if choice == "+" {
			s.pickerOpen("all")
			return
		}
		if choice == "@method" {
			s.pickerOpen("methods")
			return
		}
		for i, code := range s.languages {
			if code == choice {
				s.language = i
				s.startLanguage()
				break
			}
		}
	case "all":
		s.languages = textinput.Languages(strings.Join(append(s.languages, choice), ","))
		for i, code := range s.languages {
			if code == choice {
				s.language = i
			}
		}
		s.app.config.Update(func(c *config.Config) {
			if c.InputLanguages == nil {
				c.InputLanguages = map[string][]string{}
			}
			c.InputLanguages[s.archiveID] = append([]string(nil), s.languages...)
		})
		s.startLanguage()
	case "methods":
		for i, method := range s.profile.Methods {
			if method == choice {
				s.methodIndex = i
				break
			}
		}
		s.picker = ""
		s.focus = 0
		s.startMethod()
		s.persist()
	}
}

func (s *searchState) submit() {
	if s.systemPreedit {
		return
	}
	if s.pendingSubmit || s.pendingLanguage >= 0 || s.pendingPicker != "" {
		return
	}
	if s.session != nil {
		if s.session.Send("commit", "") {
			s.pendingSubmit = true
			s.pendingSequence = s.session.Sequence()
		}
		return
	}
	s.focusResults()
}

const liveSearchDelay = 350 * time.Millisecond

func (s *searchState) currentQuery() string { return strings.TrimSpace(s.text + s.preedit) }
func (s *searchState) cancelSearch() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.reply = nil
	s.busy = false
}
func (s *searchState) clearResults() {
	s.cancelSearch()
	s.result = nil
	s.entries = nil
	s.more = false
	s.offset = 0
}
func (s *searchState) updateQuery(now time.Time) {
	query := s.currentQuery()
	if query != s.observedQuery {
		s.observedQuery = query
		s.clearResults()
		s.searchDue = time.Time{}
		if query != "" && len(query) <= textinput.MaxTextBytes {
			s.searchDue = now.Add(liveSearchDelay)
			s.busy = true
		}
		s.showResults(nil)
	}
	if !s.searchDue.IsZero() && !now.Before(s.searchDue) {
		s.searchDue = time.Time{}
		s.performSearch(0)
	}
}
func (s *searchState) focusResults() {
	s.editing = false
	s.visible = false
	s.picker = ""
	s.systemPreedit = false
	s.stickDirection = 0
	s.lastDirection = 0
	sdl.StopTextInput()
	if s.session != nil {
		s.session.Close()
		s.session = nil
	}
	query := s.currentQuery()
	if query != s.observedQuery || query == "" {
		s.clearResults()
	}
	s.observedQuery = query
	s.searchDue = time.Time{}
	if query == "" {
		s.showResults(nil)
	}
	if query != "" && (s.issuedQuery != query || s.reply == nil && s.result == nil) {
		s.performSearch(0)
	}
	s.updateHelp()
	s.refresh()
}
func (s *searchState) toggleFocus() {
	if s.editing {
		s.submit()
	} else {
		s.focusInput(s.forcedKeyboard)
	}
}
func (s *searchState) performSearch(offset int) {
	query := s.currentQuery()
	if query == "" || len(query) > textinput.MaxTextBytes {
		return
	}
	s.cancelSearch()
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.reply = make(chan textinput.Results, 1)
	reply := s.reply
	file := s.app.loader.zimReader.FilePath()
	s.offset = offset
	s.busy = true
	s.issuedQuery = query
	s.showResults(s.result)
	go func() { reply <- textinput.Search(ctx, file, query, offset) }()
}

// One dominant axis prevents diagonal jitter. Repetition ranges from 400 ms
// near the deadzone to 85 ms at full deflection and continues without events.
func (s *searchState) updateStick(now time.Time) {
	x, xs := analogDirection(s.app.gamepad.LeftX())
	y, ys := analogDirection(s.app.gamepad.LeftY())
	direction := 0
	strength := float64(0)
	horizontal := xs >= ys
	if s.stickDirection == 1 || s.stickDirection == 2 {
		horizontal = xs >= ys*0.85
	}
	if s.stickDirection == 3 || s.stickDirection == 4 {
		horizontal = !(ys >= xs*0.85)
	}
	if horizontal && x != 0 {
		direction = 2
		if x < 0 {
			direction = 1
		}
		strength = xs
	} else if y != 0 {
		direction = 4
		if y < 0 {
			direction = 3
		}
		strength = ys
	}
	if direction == 0 {
		s.stickDirection = 0
		return
	}
	interval := time.Duration(400-315*strength*strength) * time.Millisecond
	if direction != s.stickDirection || now.Sub(s.stickRepeat) >= interval {
		s.stickDirection = direction
		s.stickRepeat = now
		s.move(direction)
	}
}
func (s *searchState) showResults(result *textinput.Results) {
	doc := &document.Document{Blocks: []document.Block{&document.Heading{Level: 1, Content: s.tr("search.title")}}}
	add := func(value string) {
		doc.Blocks = append(doc.Blocks, &document.Paragraph{Inlines: []document.Inline{&document.Text{Content: value}}})
	}
	if s.text != "" || s.preedit != "" {
		add(s.text + s.preedit)
	}
	if result != nil && s.busy {
		add(s.tr("search.loading"))
	}
	if result == nil {
		if s.busy {
			add(s.tr("search.loading"))
		} else {
			add(s.tr(s.helpKey()))
		}
	} else if result.Err != nil {
		add(result.Err.Error())
	} else {
		if len(result.Entries) == 0 {
			add(s.tr("search.empty"))
		}
		for i, entry := range result.Entries {
			doc.Blocks = append(doc.Blocks, &document.Link{URL: fmt.Sprintf("search:result:%d", i), Label: entry.Title})
		}
		if s.offset > 0 {
			doc.Blocks = append(doc.Blocks, &document.Link{URL: "search:previous", Label: "← " + s.tr("search.previous")})
		}
		if s.more {
			doc.Blocks = append(doc.Blocks, &document.Link{URL: "search:next", Label: s.tr("search.next") + " →"})
		}
	}
	s.app.viewer.SetDocument(doc)
	s.updateHelp()
	s.app.viewer.Relayout()
	if s.app.links.LinkCount() > 0 {
		s.app.links.SetSelectedLinkIndex(0)
	}
}
func (s *searchState) helpKey() string {
	if s.app.gamepad.Controller() == nil {
		if s.editing && s.visible {
			return "search.kbd_help"
		}
		return "search.desktop_help"
	}
	if s.picker != "" {
		return "search.picker_help"
	}
	if s.editing && s.visible {
		return "search.help"
	}
	if s.editing {
		return "search.typing_help"
	}
	return "search.results_help"
}
func (s *searchState) updateHelp() {
	if s.app.viewer == nil || s.app.config == nil {
		return
	}
	s.app.viewer.SetStatusOverride(s.tr(s.helpKey()))
}
func (s *searchState) openSelected() {
	target := s.app.links.SelectedLinkURL()
	switch target {
	case "search:next":
		s.performSearch(s.offset + 10)
		return
	case "search:previous":
		s.performSearch(max(0, s.offset-10))
		return
	}
	var index int
	if _, err := fmt.Sscanf(target, "search:result:%d", &index); err != nil || index < 0 || index >= len(s.entries) {
		return
	}
	entry := s.entries[index]
	file := s.app.loader.zimReader.FilePath()
	s.release()
	if err := s.app.loader.displayArticle(articleLocation{File: file, Entry: entry.Path, ZIM: true}, false); err != nil {
		s.status = err.Error()
		s.showResults(&textinput.Results{Err: err})
		return
	}
	s.app.search = nil
}
func (s *searchState) process(event sdl.Event) {
	switch e := event.(type) {
	case *sdl.QuitEvent:
		s.app.running.Store(false)
	case *sdl.TextInputEvent:
		if s.editing {
			s.systemPreedit = false
			if !s.forcedKeyboard {
				s.visible = false
			}
			// SDL text has already passed through the physical keyboard's OS IME.
			s.send("text", e.GetText())
		}
	case *sdl.TextEditingEvent: // The OS IME owns its preedit until TextInput commits it.
		if s.editing {
			if !s.forcedKeyboard {
				s.visible = false
			}
			s.preedit = e.GetText()
			s.systemPreedit = s.preedit != ""
		}
	case *sdl.KeyboardEvent:
		if e.Type != sdl.KEYDOWN {
			return
		}
		switch e.Keysym.Scancode {
		case sdl.SCANCODE_ESCAPE:
			s.app.closeSearch(true)
		case sdl.SCANCODE_F3:
			s.focusInput(true)
		case sdl.SCANCODE_LEFT:
			s.move(1)
		case sdl.SCANCODE_RIGHT:
			s.move(2)
		case sdl.SCANCODE_UP:
			s.move(3)
		case sdl.SCANCODE_DOWN:
			s.move(4)
		case sdl.SCANCODE_BACKSPACE:
			if s.editing && !s.systemPreedit {
				s.send("backspace", "")
			}
		case sdl.SCANCODE_RETURN, sdl.SCANCODE_KP_ENTER:
			if s.systemPreedit {
				return
			}
			if e.Repeat == 0 {
				if s.visible {
					s.activate()
				} else if s.editing {
					s.submit()
				} else {
					s.openSelected()
				}
			}
		case sdl.SCANCODE_TAB:
			if s.visible {
				s.changeLanguage(1)
			}
		}

	case *sdl.ControllerDeviceEvent:
		s.app.gamepad.TranslateEvent(event, s.app.mode)
	case *sdl.ControllerAxisEvent:
		s.app.gamepad.TranslateEvent(event, s.app.mode)

	case *sdl.ControllerButtonEvent:
		if e.State != sdl.PRESSED {
			return
		}
		switch e.Button {
		case sdl.CONTROLLER_BUTTON_B: // Physical A: confirm, as in article navigation.
			s.activate()
		case sdl.CONTROLLER_BUTTON_A: // Physical B: delete while typing, back in results.
			if s.picker != "" {
				s.picker = ""
			} else if s.editing {
				s.send("backspace", "")
			} else {
				s.app.closeSearch(true)
			}
		case sdl.CONTROLLER_BUTTON_X:
			if s.picker != "" {
				s.picker = ""
			} else if s.visible {
				if s.layer >= 2 {
					s.changeLayer(5 - s.layer)
				} else {
					s.changeLayer(1 - s.layer)
				}
			} else {
				s.focusInput(false)
			}
		case sdl.CONTROLLER_BUTTON_START:
			s.toggleFocus()
		case sdl.CONTROLLER_BUTTON_BACK:
			s.focusInput(false)
		case sdl.CONTROLLER_BUTTON_LEFTSHOULDER:
			if !s.editing {
				s.focusInput(false)
			}
			s.send("backspace", "")
		case sdl.CONTROLLER_BUTTON_RIGHTSHOULDER:
			if s.picker != "" {
				s.pagePicker(1)
			} else if s.visible {
				s.changeLanguage(1)
			} else if s.more {
				s.performSearch(s.offset + 10)
			}
		case sdl.CONTROLLER_BUTTON_DPAD_LEFT:
			s.move(1)
		case sdl.CONTROLLER_BUTTON_DPAD_RIGHT:
			s.move(2)
		case sdl.CONTROLLER_BUTTON_DPAD_UP:
			s.move(3)
		case sdl.CONTROLLER_BUTTON_DPAD_DOWN:
			s.move(4)
		case sdl.CONTROLLER_BUTTON_GUIDE:
			s.app.openSettings()
		}
	case *sdl.WindowEvent:
		if e.Event == sdl.WINDOWEVENT_RESIZED || e.Event == sdl.WINDOWEVENT_SIZE_CHANGED {
			s.app.viewer.Relayout()
		}
	case *sdl.MouseButtonEvent:
		if e.Type != sdl.MOUSEBUTTONDOWN {
			return
		}
		if s.visible {
			g := s.app.viewer.(keyboardViewer).KeyboardGeometry()
			idx := g.Hit(e.X, e.Y)
			if idx >= 0 {
				s.focus = idx

				s.activate()
			} else if e.Y < g.Y {
				if url := s.app.links.HandleClick(e.X, e.Y); url != "" {
					s.openSelected()
				}
			}
		} else {
			if url := s.app.links.HandleClick(e.X, e.Y); url != "" {
				s.openSelected()
			}
		}
	}
	if s.app.search == s {
		s.updateHelp()
		s.refresh()
	}
}
