package tailer

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

type Tailer struct {
	// The file we're tailing
	File string
	// Read only chan
	// The event will be a single line of text to get started, but will probably
	// become something more complex later
	Events <-chan string
	// Parent folder, computed
	folder string
	// Folder and file watchers
	watcher *fsnotify.Watcher
	// File descriptor and buffer to read the content
	fd     *os.File
	reader *bufio.Reader
	// Internal event manager
	events chan string
}

func NewTailer(file string) (*Tailer, error) {
	t := &Tailer{File: file}
	t.folder = filepath.Dir(file)

	info, err := os.Stat(t.folder)
	if err != nil {
		return nil, fmt.Errorf("could not open %s: %v", t.folder, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", t.folder)
	}

	ch := make(chan string, 64)
	t.Events = ch
	t.events = ch

	t.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("could not create the watcher: %v", err)
	}
	t.watcher.Add(t.folder)

	return t, nil
}

func (t *Tailer) watchFile(skipToTheEnd bool) error {
	if t.fd != nil {
		t.fd.Close()
	}

	fd, err := os.OpenFile(t.File, os.O_RDONLY, 0644)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	t.fd = fd
	t.reader = bufio.NewReader(t.fd)

	if skipToTheEnd {
		t.fd.Seek(0, io.SeekEnd)
	}

	t.watcher.Add(t.File)
	slog.Info("tailing file", "file", t.File, "from_end", skipToTheEnd)

	return nil
}

func (t *Tailer) readAndEmit() {
	// We need to detect if the file has been truncated
	offset, _ := t.fd.Seek(0, io.SeekCurrent)
	trueOffset := offset - int64(t.reader.Buffered())

	info, err := t.fd.Stat()
	if err != nil {
		slog.Debug("failed to stat file", "file", t.File, "err", err)
		return
	}

	slog.Debug("offset check", "file", t.File, "offset", trueOffset, "size", info.Size())
	if trueOffset > info.Size() {
		// File was truncated (copytruncate rotation) — rewind
		slog.Warn("file truncated, rewinding", "file", t.File)
		t.fd.Seek(0, io.SeekStart)
		t.reader.Reset(t.fd)
	}

	for {
		line, err := t.reader.ReadString('\n')

		if err == io.EOF {
			return
		}
		if err != nil {
			slog.Warn("failed to read file", "file", t.File, "err", err)
			return
		}

		line = strings.TrimRight(line, "\n")
		t.events <- line
	}
}

func (t *Tailer) Run() error {
	err := t.watchFile(true)
	if err != nil {
		return fmt.Errorf("could not start tailer: %v", err)
	}

	for event := range t.watcher.Events {
		if event.Has(fsnotify.Create) && event.Name == t.File {
			slog.Info("file appeared, tailing from start", "file", event.Name)
			err = t.watchFile(false)
			if err != nil {
				return fmt.Errorf("could not watch file: %v", err)
			}
			t.readAndEmit()
		} else if event.Has(fsnotify.Write) && event.Name == t.File {
			t.readAndEmit()
		}
	}

	return nil
}
