# Session Management

Sessions track the history of interactions between the user and the AI, enabling context continuity across multiple turns.

## Store

The `Store` type (`internal/session/store.go`) manages session persistence:

```go
type Store struct {
    workingDir   string
    rootDir      string
    namespaceDir string
}

func NewStore(workingDir string) (*Store, error)
```

Sessions are stored under `~/.keen/sessions/` with namespace subdirectories.

## Session Directory Structure

```
~/.keen/sessions/
├── project-path-hash/          # Namespace by working directory
│   ├── session-id-1/           # Each session has its own directory
│   │   └── transcript_events.jsonl
│   ├── session-id-2/
│   │   └── transcript_events.jsonl
│   └── ...
```

### Path Generation

```go
// internal/session/path.go
func sessionsRootDir() (string, error)
func namespaceDirName(workingDir string) string  // e.g., "my-project-a1b2c3d4e5"
func sessionDirName(sessionID string) string
```

Namespace names are derived from the working directory:
- Sanitized: `/` → `-`, `\` → `-`
- Hashed: SHA1 prefix (10 chars) for uniqueness

## Session Model

```go
type Session struct {
    ID             string
    CreatedAt      time.Time
    Directory      string
    TranscriptPath string
    nextSeq        uint64
}

type LoadedSession struct {
    Summary Summary
    Events  []Event
    Session *Session
}
```

## Events

Sessions are event-sourced. Each interaction is recorded as an event in a JSONL transcript file:

```go
type Event struct {
    Seq  uint64    `json:"seq"`
    Kind EventKind `json:"kind"`
    // Payload based on Kind...
}
```

### Event Kinds

| Kind | Description |
|------|-------------|
| `session_started` | Session created with metadata |
| `user_message` | User input |
| `assistant_turn` | AI response with transcript |
| `compaction_applied` | Context compaction applied |

### Event Payloads

```go
type SessionStartedPayload struct {
    SessionID string    `json:"session_id"`
    CreatedAt time.Time `json:"created_at"`
    CWD       string    `json:"cwd"`
}

type MessagePayload struct {
    Content string `json:"content"`
}

type AssistantTurnPayload struct {
    Transcript  []TranscriptItem `json:"transcript,omitempty"`
    Message     string           `json:"message,omitempty"`
    TurnMemory  *llm.TurnMemory  `json:"turn_memory,omitempty"`
    Interrupted bool             `json:"interrupted,omitempty"`
    Error       string           `json:"error,omitempty"`
}
```

### Transcript Items

Assistant turns include a transcript of what happened:

```go
type TranscriptItem struct {
    Kind      TranscriptItemKind `json:"kind"`
    Content   string             `json:"content,omitempty"`
    ToolStart *ToolStartPayload  `json:"tool_start,omitempty"`
    ToolEnd   *ToolEndPayload    `json:"tool_end,omitempty"`
    Bash      *BashPayload       `json:"bash,omitempty"`
    Diff      *DiffPayload       `json:"diff,omitempty"`
}

const (
    TranscriptItemText      TranscriptItemKind = "text"
    TranscriptItemReasoning TranscriptItemKind = "reasoning"
    TranscriptItemToolStart TranscriptItemKind = "tool_start"
    TranscriptItemToolEnd   TranscriptItemKind = "tool_end"
    TranscriptItemBash      TranscriptItemKind = "bash"
    TranscriptItemDiff      TranscriptItemKind = "diff"
)
```

## Store Operations

### Create Session

```go
func (s *Store) Create() (*Session, error)
```

Creates a new session directory and writes a `session_started` event.

### Append Event

```go
func (s *Store) Append(session *Session, event Event) error
```

Appends an event to the transcript JSONL file. Each event gets an incrementing sequence number.

### List Sessions

```go
func (s *Store) List() ([]Summary, error)
```

Returns summaries of all sessions in the namespace, sorted by most recently updated.

### Load Session

```go
func (s *Store) Load(summary Summary) (*LoadedSession, error)
```

Loads all events from a session's transcript file.

## Summary

Summaries provide lightweight session metadata for listing:

```go
type Summary struct {
    ID              string
    CreatedAt       time.Time
    UpdatedAt       time.Time
    LastUserMessage string
    Directory       string
    TranscriptPath  string
    LastSeq         uint64
}
```

## Conversation Projection

The `projection.go` file provides utilities to rebuild conversation history from events:

```go
func BuildConversation(events []Event) []llm.Message
```

This reconstructs the message list needed for LLM context, handling:
- `user_message` events → user role messages
- `assistant_turn` events → assistant role messages (with turn memory)
- `compaction_applied` events → replaces messages with compacted version

## Session ID Generation

Session IDs are version 4 UUIDs:

```go
func generateSessionID() (string, error)
```

Generated using `crypto/rand` with appropriate version/variant bits set.

## Transcript Format

The transcript is a JSONL file (JSON Lines) where each line is a single JSON event:

```
{"seq":1,"kind":"session_started",...}
{"seq":2,"kind":"user_message",...}
{"seq":3,"kind":"assistant_turn",...}
...
```

This format allows efficient appending and partial loading.