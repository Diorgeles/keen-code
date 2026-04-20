package repl

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	tea "charm.land/bubbletea/v2"
	reploutput "github.com/user/keen-code/internal/cli/repl/output"
	replpermissions "github.com/user/keen-code/internal/cli/repl/permissions"
	repltheme "github.com/user/keen-code/internal/cli/repl/theme"
	"github.com/user/keen-code/internal/llm"
)

const (
	keyEnter     = "enter"
	keyCtrlC     = "ctrl+c"
	keyCtrlD     = "ctrl+d"
	keyEsc       = "esc"
	keyTab       = "tab"
	keyUp        = "up"
	keyDown      = "down"
	keyPageUp    = "pgup"
	keyPageDown  = "pgdown"
	keyHome      = "home"
	keyEnd       = "end"
	keyShiftUp   = "shift+up"
	keyShiftDown = "shift+down"
)

func (m *replModel) handleLLMChunk(chunk string) (replModel, tea.Cmd) {
	m.streamHandler.HandleChunk(chunk)
	m.updateViewportContent()
	if !m.userScrolled {
		m.viewport.GotoBottom()
	}
	return *m, m.waitForAsyncEvent()
}

func (m *replModel) handleLLMReasoningChunk(chunk string) (replModel, tea.Cmd) {
	m.streamHandler.HandleReasoningChunk(chunk)
	m.updateViewportContent()
	if !m.userScrolled {
		m.viewport.GotoBottom()
	}
	return *m, m.waitForAsyncEvent()
}

func (m *replModel) handleLLMDone() (replModel, tea.Cmd) {
	if m.isCompacting {
		return m.handleCompactionDone()
	}
	m.streamHandler.finalizeAssistantContent()
	segments := cloneStreamSegments(m.streamHandler.segments)
	m.showSpinner = false
	m.clearStreamCancel()
	m.adjustTextareaHeight()
	responseLines, response := m.streamHandler.HandleDone()
	assistantMessage := llm.Message{
		Role:       llm.RoleAssistant,
		Content:    response,
		TurnMemory: m.consumeTurnMemory(),
	}
	m.appState.AppendMessage(assistantMessage)
	// m.logAppStateMessages("assistant_turn_completed")
	if err := m.sessions.appendAssistantTurn(segments, assistantMessage, false, ""); err != nil {
		m.handleSessionPersistenceError(err)
	}
	m.refreshContextStatus(false)
	for _, line := range responseLines {
		m.output.AddLine(line)
	}
	m.output.AddEmptyLine()
	m.updateViewportContent()
	if !m.userScrolled {
		m.viewport.GotoBottom()
	}
	return *m, nil
}

func (m *replModel) handleLLMError(err error) (replModel, tea.Cmd) {
	if m.isCompacting {
		return m.handleCompactionError(err)
	}
	m.streamHandler.finalizeAssistantContent()
	segments := cloneStreamSegments(m.streamHandler.segments)
	m.showSpinner = false
	m.clearStreamCancel()
	m.clearTurnMemory()
	m.adjustTextareaHeight()
	pendingLines, errMsg := m.streamHandler.HandleError(err)
	if persistErr := m.sessions.appendAssistantTurn(segments, llm.Message{}, false, errMsg); persistErr != nil {
		m.handleSessionPersistenceError(persistErr)
	}
	for _, line := range pendingLines {
		m.output.AddLine(line)
	}
	if errors.Is(err, context.Canceled) {
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m, nil
	}
	m.output.AddError(errMsg, repltheme.ErrorStyle)
	m.updateViewportContent()
	m.viewport.GotoBottom()
	return *m, nil
}

func (m *replModel) handleCompactionDone() (replModel, tea.Cmd) {
	m.streamHandler.finalizeAssistantContent()
	segments := cloneStreamSegments(m.streamHandler.segments)
	responseLines, summary := m.streamHandler.HandleDone()
	m.isCompacting = false
	m.showSpinner = false
	m.compactionCancel = nil
	m.clearStreamCancel()
	if err := m.appState.ApplyCompaction(summary); err != nil {
		return m.handleCompactionError(err)
	}
	m.refreshContextStatus(false)
	for _, line := range responseLines {
		m.output.AddLine(line)
	}
	if len(responseLines) > 0 {
		m.output.AddEmptyLine()
	}
	if err := m.sessions.appendCompaction(segments, m.appState.GetMessages(), "Context compacted."); err != nil {
		m.handleSessionPersistenceError(err)
	}
	m.adjustTextareaHeight()
	m.updateViewportContent()
	m.viewport.GotoBottom()
	return *m, nil
}

func (m *replModel) handleCompactionError(err error) (replModel, tea.Cmd) {
	if m.streamHandler != nil && m.streamHandler.IsActive() {
		responseLines, _ := m.streamHandler.HandleError(err)
		for _, line := range responseLines {
			m.output.AddLine(line)
		}
		if len(responseLines) > 0 {
			m.output.AddEmptyLine()
		}
	}
	m.isCompacting = false
	m.showSpinner = false
	m.compactionCancel = nil
	m.clearStreamCancel()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			reploutput.AddCompactionCancelledStatus(m.output, "Compaction cancelled.")
		} else {
			status := "Compaction failed: " + err.Error()
			reploutput.AddCompactionErrorStatus(m.output, status)
		}
	}
	m.adjustTextareaHeight()
	m.refreshContextStatus(false)
	m.updateViewportContent()
	m.viewport.GotoBottom()
	return *m, nil
}

func (m *replModel) handleToolStart(toolCall *llm.ToolCall) (replModel, tea.Cmd) {
	if toolCall.Name == "bash" {
		command, _ := toolCall.Input["command"].(string)
		summary, _ := toolCall.Input["summary"].(string)
		m.streamHandler.HandleBashStart(command, summary)
	} else {
		m.streamHandler.HandleToolStart(toolCall)
	}
	m.updateViewportContent()
	if !m.userScrolled {
		m.viewport.GotoBottom()
	}
	return *m, m.waitForAsyncEvent()
}

func (m *replModel) handleToolEnd(toolCall *llm.ToolCall) (replModel, tea.Cmd) {
	m.recordToolMemory(toolCall)
	if toolCall.Name == "bash" {
		m.streamHandler.HandleBashEnd(toolCall)
	} else {
		m.streamHandler.HandleToolEnd(toolCall)
		m.loadingText = nextLoadingText()
		m.streamHandler.SetLoadingText(m.loadingText)
	}
	m.updateViewportContent()
	if !m.userScrolled {
		m.viewport.GotoBottom()
	}
	return *m, m.waitForAsyncEvent()
}

func (m *replModel) handleKeyMsg(msg tea.Msg) (replModel, tea.Cmd) {
	if m.sessionPicker != nil {
		return m.handleSessionPickerKeyMsg(msg)
	}

	if m.modelSelection != nil {
		var cmd tea.Cmd
		m.modelSelection, cmd = m.modelSelection.Update(msg)
		m.updateViewportContent()
		return *m, cmd
	}

	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return *m, nil
	}

	if m.isCompacting {
		if keyMsg.String() == keyEsc && m.compactionCancel != nil {
			m.compactionCancel()
			m.compactionCancel = nil
		}
		return *m, nil
	}

	if m.streamHandler != nil && m.streamHandler.HasPendingPermission() {
		switch keyMsg.String() {
		case "up", "k", "down", "j", keyEnter, keyEsc:
			return m.handlePermissionKeyMsg(keyMsg)
		}
	}

	if m.suggestion.Visible() {
		switch keyMsg.String() {
		case keyEnter, keyTab:
			if cur := m.suggestion.Current(); cur != nil {
				m.textarea.SetValue(cur.Name)
			} else if first := m.suggestion.First(); first != nil {
				m.textarea.SetValue(first.Name)
			}
			if keyMsg.String() == keyEnter {
				m.suggestion.Refresh("")
			} else {
				m.suggestion.Refresh(m.textarea.Value())
			}
			m.adjustTextareaHeight()
			return *m, nil
		case keyUp, keyShiftUp:
			m.suggestion.MoveUp()
			return *m, nil
		case keyDown, keyShiftDown:
			m.suggestion.MoveDown()
			return *m, nil
		case keyEsc:
			if m.streamHandler == nil || !m.streamHandler.IsActive() {
				m.suggestion.Refresh("")
				return *m, nil
			}
		}
	} else if keyMsg.String() == keyTab {
		return *m, nil
	}

	switch keyMsg.String() {
	case keyEnter:
		return m.handleEnterKey()
	case keyCtrlC, keyCtrlD:
		if m.textarea.Value() != "" {
			m.textarea.Reset()
			m.adjustTextareaHeight()
			return *m, nil
		}
		m.quitting = true
		return *m, tea.Quit
	case keyEsc:
		if m.streamHandler != nil && m.streamHandler.IsActive() {
			m.interruptStream(interruptedPromptText)
		}
		return *m, nil
	case keyUp, keyShiftUp:
		if m.isAtTopOfInput() {
			m.viewport.ScrollUp(1)
			m.userScrolled = !m.viewport.AtBottom()
			return *m, nil
		}
	case keyDown, keyShiftDown:
		if m.isAtBottomOfInput() {
			m.viewport.ScrollDown(1)
			m.userScrolled = !m.viewport.AtBottom()
			return *m, nil
		}
	case keyPageUp:
		m.viewport.HalfPageUp()
		m.userScrolled = !m.viewport.AtBottom()
		return *m, nil
	case keyPageDown:
		m.viewport.HalfPageDown()
		m.userScrolled = !m.viewport.AtBottom()
		return *m, nil
	case keyHome:
		m.viewport.GotoTop()
		m.userScrolled = true
		return *m, nil
	case keyEnd:
		m.viewport.GotoBottom()
		m.userScrolled = false
		return *m, nil
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(keyMsg)
	m.suggestion.Refresh(m.textarea.Value())
	m.adjustTextareaHeight()
	return *m, cmd
}

func (m *replModel) interruptStream(message string) {
	if m.streamCancel != nil {
		m.streamCancel()
		m.clearStreamCancel()
	}

	m.showSpinner = false

	m.streamHandler.finalizeAssistantContent()
	partialResponse := m.streamHandler.GetResponse()
	segments := cloneStreamSegments(m.streamHandler.segments)
	interruptedMessage := ""
	if partialResponse != "" {
		interruptedMessage = partialResponse + "\n\n[Response interrupted by user]"
		m.appState.AppendMessage(llm.Message{
			Role:    llm.RoleAssistant,
			Content: interruptedMessage,
		})
		// m.logAppStateMessages("assistant_turn_interrupted")
	}
	if err := m.sessions.appendAssistantTurn(segments, llm.Message{
		Role:    llm.RoleAssistant,
		Content: interruptedMessage,
	}, true, ""); err != nil {
		m.handleSessionPersistenceError(err)
	}
	m.clearTurnMemory()

	for _, line := range m.streamHandler.HandleInterrupt() {
		m.output.AddLine(line)
	}
	m.output.AddStyledLine("\n  "+message, repltheme.InterruptedStyle)
	m.output.AddEmptyLine()
	m.adjustTextareaHeight()
	m.updateViewportContent()
	m.viewport.GotoBottom()
}

func (m *replModel) logAppStateMessages(reason string) {
	if m == nil || m.appState == nil {
		return
	}

	payload, err := json.MarshalIndent(m.appState.GetMessages(), "", "  ")
	if err != nil {
		slog.Debug("AppState messages", "reason", reason, "marshal_error", err.Error())
		return
	}

	slog.Debug("AppState messages", "reason", reason, "messages", string(payload))
}

func (m *replModel) handleSessionPickerKeyMsg(msg tea.Msg) (replModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok || m.sessionPicker == nil {
		return *m, nil
	}

	switch keyMsg.String() {
	case keyUp, "k", keyShiftUp:
		m.sessionPicker.Move(-1)
		m.updateViewportContent()
	case keyDown, "j", keyShiftDown:
		m.sessionPicker.Move(1)
		m.updateViewportContent()
	case keyEnter:
		selected := m.sessionPicker.Current()
		if selected == nil {
			return *m, nil
		}
		loaded, err := m.sessions.load(*selected)
		if err != nil {
			m.sessionPicker = nil
			m.handleSessionPersistenceError(err)
			m.updateViewportContent()
			m.viewport.GotoBottom()
			return *m, nil
		}
		m.replayLoadedSession(loaded)
	case keyEsc:
		m.sessionPicker = nil
		m.updateViewportContent()
		m.viewport.GotoBottom()
	}

	return *m, nil
}

func (m *replModel) handlePermissionKeyMsg(msg tea.KeyPressMsg) (replModel, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.streamHandler.MovePendingCursor(-1)
		m.updateViewportContent()
		if !m.userScrolled {
			m.viewport.GotoBottom()
		}
	case "down", "j":
		m.streamHandler.MovePendingCursor(1)
		m.updateViewportContent()
		if !m.userScrolled {
			m.viewport.GotoBottom()
		}
	case keyEnter:
		req := m.streamHandler.GetPendingPermissionRequest()
		if req == nil {
			return *m, nil
		}
		choice := m.streamHandler.GetPendingChoice()
		var status replpermissions.Status
		switch choice {
		case replpermissions.ChoiceAllow:
			status = replpermissions.StatusAllowed
		case replpermissions.ChoiceAllowSession:
			status = replpermissions.StatusAllowedSession
		case replpermissions.ChoiceDeny:
			status = replpermissions.StatusDenied
		}
		m.streamHandler.ResolvePendingPermission(status)
		m.permissionRequester.SendResponse(choice, req.ToolName)
		m.updateViewportContent()
		if !m.userScrolled {
			m.viewport.GotoBottom()
		}
	case keyEsc:
		req := m.streamHandler.GetPendingPermissionRequest()
		if req == nil {
			return *m, nil
		}
		m.streamHandler.ResolvePendingPermission(replpermissions.StatusDenied)
		m.permissionRequester.SendResponse(replpermissions.ChoiceDeny, req.ToolName)
		m.updateViewportContent()
		if !m.userScrolled {
			m.viewport.GotoBottom()
		}
	}
	return *m, nil
}

func (m replModel) handleLLMStreamMsg(msg tea.Msg) (replModel, tea.Cmd, bool) {
	if m.streamHandler == nil || !m.streamHandler.IsActive() {
		switch msg.(type) {
		case llmChunkMsg, llmReasoningChunkMsg, llmDoneMsg, llmErrorMsg, llmToolStartMsg, llmToolEndMsg:
			return m, nil, true
		}
	}

	switch msg := msg.(type) {
	case llmChunkMsg:
		updated, cmd := m.handleLLMChunk(string(msg))
		return updated, cmd, true
	case llmReasoningChunkMsg:
		updated, cmd := m.handleLLMReasoningChunk(string(msg))
		return updated, cmd, true
	case llmDoneMsg:
		updated, cmd := m.handleLLMDone()
		return updated, cmd, true
	case llmErrorMsg:
		updated, cmd := m.handleLLMError(msg.err)
		return updated, cmd, true
	case llmToolStartMsg:
		updated, cmd := m.handleToolStart(msg.toolCall)
		return updated, cmd, true
	case llmToolEndMsg:
		updated, cmd := m.handleToolEnd(msg.toolCall)
		return updated, cmd, true
	default:
		return m, nil, false
	}
}
