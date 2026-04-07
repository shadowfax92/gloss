package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"gloss/internal/paths"
)

type Client struct {
	Info DaemonInfo
	http *http.Client
}

func newClient(info DaemonInfo) *Client {
	return &Client{
		Info: info,
		http: &http.Client{Timeout: 5 * time.Second},
	}
}

// EnsureRunning returns a Client pointing at a live daemon, spawning one if
// necessary. Safe to call concurrently from multiple processes — uses a
// file lock around the spawn critical section.
func EnsureRunning() (*Client, error) {
	if err := paths.EnsureAll(); err != nil {
		return nil, err
	}
	lock, err := acquireLock()
	if err != nil {
		return nil, err
	}
	defer releaseLock(lock)

	if info, ok := readServerJSON(); ok {
		c := newClient(info)
		if processAlive(info.PID) && c.Healthz() == nil {
			return c, nil
		}
		_ = os.Remove(paths.ServerJSON())
	}

	if err := spawnDaemon(); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if info, ok := readServerJSON(); ok {
			c := newClient(info)
			if c.Healthz() == nil {
				return c, nil
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil, fmt.Errorf("daemon failed to start; see %s", paths.DaemonLog())
}

func (c *Client) Healthz() error {
	url := fmt.Sprintf("http://127.0.0.1:%d/healthz", c.Info.Port)
	resp, err := c.http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz: %s", resp.Status)
	}
	return nil
}

type OpenResult struct {
	FolderID string `json:"folder_id"`
	FileRel  string `json:"file_rel,omitempty"`
}

func (c *Client) Open(folderAbs, fileAbs string) (*OpenResult, error) {
	body, _ := json.Marshal(map[string]string{"folder": folderAbs, "file": fileAbs})
	req, _ := http.NewRequest("POST", c.url("/api/open"), bytes.NewReader(body))
	req.Header.Set("X-Gloss-Token", c.Info.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("open: %s: %s", resp.Status, string(msg))
	}
	var out OpenResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) BrowserURL(folderID, fileRel string) string {
	v := url.Values{}
	v.Set("folder", folderID)
	if fileRel != "" {
		v.Set("file", fileRel)
	}
	v.Set("t", c.Info.Token)
	return fmt.Sprintf("http://127.0.0.1:%d/?%s", c.Info.Port, v.Encode())
}

func (c *Client) Shutdown() error {
	req, _ := http.NewRequest("POST", c.url("/api/shutdown"), nil)
	req.Header.Set("X-Gloss-Token", c.Info.Token)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) url(p string) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", c.Info.Port, p)
}

func readServerJSON() (DaemonInfo, bool) {
	data, err := os.ReadFile(paths.ServerJSON())
	if err != nil {
		return DaemonInfo{}, false
	}
	var info DaemonInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return DaemonInfo{}, false
	}
	if info.Version != daemonProtocolVersion {
		return DaemonInfo{}, false
	}
	return info, true
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

func spawnDaemon() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(paths.DaemonLog()), 0755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(paths.DaemonLog(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	cmd := exec.Command(self, "_serve", "--detached")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.SysProcAttr = sysProcAttrDetached()
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return err
	}
	go func() {
		_ = cmd.Wait()
		logFile.Close()
	}()
	return nil
}

func sysProcAttrDetached() *syscall.SysProcAttr {
	if runtime.GOOS == "windows" {
		return nil
	}
	return &syscall.SysProcAttr{Setsid: true}
}

type fileLock struct {
	f *os.File
}

func acquireLock() (*fileLock, error) {
	if err := os.MkdirAll(filepath.Dir(paths.ServerLock()), 0755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(paths.ServerLock(), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return &fileLock{f: f}, nil
}

func releaseLock(l *fileLock) {
	if l == nil || l.f == nil {
		return
	}
	syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	l.f.Close()
}

// ErrNoDaemon is returned when no daemon is running and the caller did not
// request to spawn one.
var ErrNoDaemon = errors.New("no gloss daemon running")

// Existing returns a client to a running daemon if one exists, otherwise
// ErrNoDaemon. Does not spawn.
func Existing() (*Client, error) {
	info, ok := readServerJSON()
	if !ok {
		return nil, ErrNoDaemon
	}
	c := newClient(info)
	if !processAlive(info.PID) || c.Healthz() != nil {
		return nil, ErrNoDaemon
	}
	return c, nil
}
