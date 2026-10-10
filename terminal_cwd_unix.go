//go:build darwin || linux

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// A private FIFO wakes zsh's line editor. Directory updates never enter the
// PTY input stream, so they cannot become commands in a foreground program.
// precmd applies the latest update when a foreground program finishes.
type cwdBridge struct {
	mu       sync.Mutex
	dir      string
	fifo     *os.File
	revision uint64
	closed   bool
}

func newCWDBridge(path string) (*cwdBridge, error) {
	dir, err := os.MkdirTemp("", "twintrail-terminal-*")
	if err != nil {
		return nil, err
	}
	b := &cwdBridge{dir: dir}
	fail := func(e error) (*cwdBridge, error) { b.close(); return nil, e }
	if err = syscall.Mkfifo(filepath.Join(dir, "notify"), 0600); err != nil {
		return fail(err)
	}
	b.fifo, err = os.OpenFile(filepath.Join(dir, "notify"), os.O_RDWR|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return fail(err)
	}
	files := map[string]string{".zshenv": `ZDOTDIR=${TWINTRAIL_USER_ZDOTDIR:-$HOME}
[[ -r "$ZDOTDIR/.zshenv" ]] && source "$ZDOTDIR/.zshenv"
TWINTRAIL_USER_ZDOTDIR=${ZDOTDIR:-$HOME}
ZDOTDIR=$TWINTRAIL_TERMINAL_DIR
`, ".zprofile": `[[ -r "$TWINTRAIL_USER_ZDOTDIR/.zprofile" ]] && source "$TWINTRAIL_USER_ZDOTDIR/.zprofile"
ZDOTDIR=$TWINTRAIL_TERMINAL_DIR
`, ".zshrc": zshCWDIntegration}
	for name, data := range files {
		if err = os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			return fail(err)
		}
	}
	if err = b.setDir(path, 0); err != nil {
		return fail(err)
	}
	return b, nil
}
func (b *cwdBridge) env() []string {
	original := os.Getenv("ZDOTDIR")
	if original == "" {
		original, _ = os.UserHomeDir()
	}
	return []string{"ZDOTDIR=" + b.dir, "TWINTRAIL_TERMINAL_DIR=" + b.dir, "TWINTRAIL_USER_ZDOTDIR=" + original}
}
func (b *cwdBridge) setDir(path string, revision uint64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return os.ErrClosed
	}
	if revision < b.revision {
		return nil
	}
	tmp, err := os.CreateTemp(b.dir, "cwd-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	_, err = fmt.Fprintf(tmp, "%d\n%s%c", revision, path, 0)
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(name, filepath.Join(b.dir, "cwd")); err != nil {
		return err
	}
	b.revision = revision
	_, err = b.fifo.Write([]byte("\n"))
	if errors.Is(err, syscall.EAGAIN) {
		return nil
	}
	return err
}
func (b *cwdBridge) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	if b.fifo != nil {
		b.fifo.Close()
	}
	os.RemoveAll(b.dir)
}

const zshCWDIntegration = `ZDOTDIR=$TWINTRAIL_USER_ZDOTDIR
[[ -r "$ZDOTDIR/.zshrc" ]] && source "$ZDOTDIR/.zshrc"
typeset -g _twintrail_terminal_dir=$TWINTRAIL_TERMINAL_DIR
unset TWINTRAIL_TERMINAL_DIR TWINTRAIL_USER_ZDOTDIR
typeset -g _twintrail_revision=-1
typeset -g _twintrail_notify_fd
exec {_twintrail_notify_fd}<>"$_twintrail_terminal_dir/notify"
_twintrail_apply_cwd() {
  local revision wanted
  { IFS= read -r revision; IFS= read -r -d '' wanted } < "$_twintrail_terminal_dir/cwd"
  if [[ $revision != $_twintrail_revision && -d $wanted ]]; then
    builtin cd -- "$wanted" && _twintrail_revision=$revision
  fi
}
_twintrail_notify_cwd() {
  local ignored
  IFS= read -r ignored <&$_twintrail_notify_fd
  _twintrail_apply_cwd
  zle reset-prompt
}
autoload -Uz add-zsh-hook
_twintrail_report_cwd() {
 local path=$PWD
 path=${path//\%/\%25}
 path=${path// /%20}
 path=${path//$'\n'/%0A}
 printf '\e]7;file://localhost%s\e\\' "$path"
}
add-zsh-hook chpwd _twintrail_report_cwd
add-zsh-hook precmd _twintrail_apply_cwd
add-zsh-hook precmd _twintrail_report_cwd
zle -F $_twintrail_notify_fd _twintrail_notify_cwd
_twintrail_apply_cwd
`
