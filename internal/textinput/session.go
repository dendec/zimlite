package textinput

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type State struct {
	Sequence           int
	Text, Preedit      string
	Candidates, Labels []string
	Candidate          int
	Err                error
}

// Session serializes IME requests off the UI thread. Results are never dropped:
// every reply is a complete snapshot and the bounded command queue rejects overload.
type Session struct {
	sent     int
	commands chan string
	states   chan State
	stop     chan struct{}
	once     sync.Once
}

func RuntimeDir() string {
	if p := os.Getenv("KIWIX_INPUT_RUNTIME_DIR"); p != "" {
		return p
	}
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "input")
}
func Helper(name string) string { return filepath.Join(RuntimeDir(), name) }
func Start(language, method, text string) *Session {
	s := &Session{commands: make(chan string, 16), states: make(chan State, 16), stop: make(chan struct{})}
	go s.run(language, method, text)
	return s
}
func (s *Session) Sequence() int { return s.sent }
func (s *Session) Close()        { s.once.Do(func() { close(s.stop) }) }
func (s *Session) Send(kind, value string) bool {
	if len(value) > MaxTextBytes {
		return false
	}
	command := kind + "\t" + hex.EncodeToString([]byte(value)) + "\n"
	select {
	case <-s.stop:
		return false
	default:
	}
	select {
	case s.commands <- command:
		s.sent++
		return true
	default:
		return false
	}
}
func (s *Session) Poll() (State, bool) {
	select {
	case st := <-s.states:
		return st, true
	default:
		return State{}, false
	}
}
func (s *Session) emit(st State) bool {
	select {
	case s.states <- st:
		return true
	case <-s.stop:
		return false
	}
}
func WorkerCommand(name string, args ...string) *exec.Cmd {
	dir := RuntimeDir()
	cmd := exec.Command(filepath.Join(dir, name), args...)
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+filepath.Join(dir, "lib"), "M17NDIR="+filepath.Join(dir, "db"), "M17N_INPUT_MODULE_DIR="+filepath.Join(dir, "lib"))
	return cmd
}

func killWorker(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		slog.Warn("failed to stop input worker", "pid", cmd.Process.Pid, "error", err)
	}
}

func (s *Session) run(language, method, text string) {
	cmd := WorkerCommand("zimlite-input-host", language, method, hex.EncodeToString([]byte(text)), RuntimeDir())
	in, err := cmd.StdinPipe()
	if err != nil {
		s.emit(State{Err: err})
		return
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		if closeErr := in.Close(); closeErr != nil {
			slog.Debug("close input worker stdin after stdout setup failure", "error", closeErr)
		}
		s.emit(State{Err: err})
		return
	}
	if err = cmd.Start(); err != nil {
		if closeErr := in.Close(); closeErr != nil {
			slog.Debug("close input worker stdin after start failure", "error", closeErr)
		}
		s.emit(State{Err: err})
		return
	}
	defer func() {
		if err := in.Close(); err != nil {
			slog.Debug("close input worker stdin", "error", err)
		}
	}()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-s.stop:
			killWorker(cmd)
		case <-done:
		}
	}()
	go WatchWorker(done, func() { killWorker(cmd) })
	defer func() {
		killWorker(cmd)
		if err := cmd.Wait(); err != nil {
			slog.Debug("input worker exited", "error", err)
		}
	}()
	scan := bufio.NewScanner(out)
	scan.Buffer(make([]byte, 4096), 128*1024)
	read := func() (State, error) {
		timer := time.AfterFunc(5*time.Second, func() { killWorker(cmd) })
		defer timer.Stop()
		if !scan.Scan() {
			if e := scan.Err(); e != nil {
				return State{}, e
			}
			return State{}, io.ErrUnexpectedEOF
		}
		return parseState(scan.Text())
	}
	st, err := read()
	if err != nil {
		s.emit(State{Err: fmt.Errorf("input %s/%s: %w", language, method, err)})
		return
	}
	if !s.emit(st) {
		return
	}
	sequence := 0
	for {
		select {
		case <-s.stop:
			return
		case command := <-s.commands:
			if strings.HasPrefix(command, "backspace\t") && st.Preedit == "" {
				command = "replace\t" + hex.EncodeToString([]byte(Backspace(st.Text))) + "\n"
			}
			if _, err = io.WriteString(in, command); err != nil {
				s.emit(State{Err: err})
				return
			}
			st, err = read()
			if err != nil {
				s.emit(State{Err: err})
				return
			}
			sequence++
			st.Sequence = sequence
			if !s.emit(st) {
				return
			}
		}
	}
}
func decode(value string) (string, error) { b, e := hex.DecodeString(value); return string(b), e }
func parseState(line string) (State, error) {
	fields := strings.Split(line, "\t")
	if len(fields) >= 2 && fields[0] == "E" {
		message, e := decode(fields[1])
		if e != nil {
			return State{}, e
		}
		return State{}, errors.New(message)
	}
	if len(fields) < 5 || fields[0] != "S" {
		return State{}, errors.New("invalid input response")
	}
	var st State
	var err error
	if st.Text, err = decode(fields[1]); err != nil {
		return st, err
	}
	if len(st.Text) > MaxTextBytes {
		return st, errors.New("input too long")
	}
	if st.Preedit, err = decode(fields[2]); err != nil {
		return st, err
	}
	if st.Candidate, err = strconv.Atoi(fields[3]); err != nil {
		return st, err
	}
	n, err := strconv.Atoi(fields[4])
	if err != nil || n < 0 || n > 8 {
		return st, errors.New("invalid candidate count")
	}
	if len(fields) < 5+n {
		return st, errors.New("missing candidates")
	}
	for _, v := range fields[5 : 5+n] {
		text, e := decode(v)
		if e != nil {
			return st, e
		}
		st.Candidates = append(st.Candidates, text)
	}
	for _, v := range fields[5+n:] {
		text, e := decode(v)
		if e != nil {
			return st, e
		}
		st.Labels = append(st.Labels, text)
	}
	return st, nil
}
