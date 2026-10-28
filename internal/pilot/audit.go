package pilot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type auditEntry struct {
	Time       time.Time      `json:"time"`
	Tool       string         `json:"tool"`
	Args       map[string]any `json:"args,omitempty"`
	PageURL    string         `json:"pageUrl,omitempty"`
	Decision   string         `json:"decision"`
	Reason     string         `json:"reason,omitempty"`
	ToolError  bool           `json:"toolError,omitempty"`
	DurationMs int64          `json:"durationMs"`
}

type auditLog struct {
	mu   sync.Mutex
	file *os.File
}

func openAuditLog(path string) (*auditLog, error) {
	if path == "" {
		return &auditLog{}, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &auditLog{file: file}, nil
}

func (a *auditLog) write(entry auditEntry) {
	if a.file == nil {
		return
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	_, _ = a.file.Write(append(line, '\n'))
}

func (a *auditLog) Close() error {
	if a.file == nil {
		return nil
	}
	return a.file.Close()
}

func auditArgs(args map[string]any) map[string]any {
	if len(args) == 0 {
		return nil
	}
	safe := make(map[string]any, len(args))
	for key, value := range args {
		if sensitiveArgs[key] {
			safe[key] = "[omitted]"
			continue
		}
		if text, ok := value.(string); ok {
			safe[key] = redact(text)
			continue
		}
		safe[key] = value
	}
	return safe
}
