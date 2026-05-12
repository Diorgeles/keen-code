package repl

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestViewportSelection_SelectedTextAcrossLines(t *testing.T) {
	var selection viewportSelection
	selection.setContent("alpha beta\ngamma delta\nthird")
	selection.start(6, 0, 0)
	selection.drag(5, 1, 0)

	if got := selection.selectedText(); got != "beta\ngamma" {
		t.Fatalf("expected selected text, got %q", got)
	}
}

func TestViewportSelection_SelectWord(t *testing.T) {
	var selection viewportSelection
	selection.setContent("hello world")

	if !selection.selectWord(7, 0, 0) {
		t.Fatal("expected word selection")
	}
	if got := selection.selectedText(); got != "world" {
		t.Fatalf("expected selected word, got %q", got)
	}
}

func TestViewportSelection_RenderHighlightsVisibleSelection(t *testing.T) {
	var selection viewportSelection
	selection.setContent("hello world")
	selection.start(0, 0, 0)
	selection.drag(5, 0, 0)

	rendered := selection.render("hello world", 20, 1, 0)
	if rendered == "hello world" {
		t.Fatal("expected rendered selection to add styling")
	}
	if !strings.Contains(rendered, "hello") {
		t.Fatalf("expected highlighted view to preserve content, got %q", rendered)
	}
}

func TestReplSelectionMouseDragSelectsWithoutCopying(t *testing.T) {
	m := newTestModel()
	m.output.AddLine("hello world")
	m.updateViewportContent()
	m.blurInput()

	updated, cmd := m.updateNormalMode(tea.MouseClickMsg(tea.Mouse{X: 0, Y: 0, Button: tea.MouseLeft}))
	if cmd != nil {
		t.Fatal("expected no command on mouse down")
	}
	updated, cmd = updated.updateNormalMode(tea.MouseMotionMsg(tea.Mouse{X: 5, Y: 0, Button: tea.MouseLeft}))
	if cmd != nil {
		t.Fatal("expected no command while dragging")
	}
	updated, cmd = updated.updateNormalMode(tea.MouseReleaseMsg(tea.Mouse{X: 5, Y: 0, Button: tea.MouseLeft}))
	if cmd != nil {
		t.Fatal("expected no copy command on mouse release")
	}
	if got := updated.selection.selectedText(); got != "hello" {
		t.Fatalf("expected selected text to remain available, got %q", got)
	}
}

func TestReplSelectionMouseClickFocusesViewport(t *testing.T) {
	m := newTestModel()
	m.output.AddLine("hello world")
	m.updateViewportContent()

	updated, cmd := m.updateNormalMode(tea.MouseClickMsg(tea.Mouse{X: 0, Y: 0, Button: tea.MouseLeft}))
	if cmd != nil {
		t.Fatal("expected no command on viewport mouse down")
	}
	if updated.textarea.Focused() {
		t.Fatal("expected viewport click to blur input")
	}
}

func TestReplSelectionCtrlCCopiesSelection(t *testing.T) {
	m := newTestModel()
	m.output.AddLine("hello world")
	m.updateViewportContent()
	m.selection.start(0, 0, 0)
	m.selection.drag(5, 0, 0)

	updated, cmd := m.updateNormalMode(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected copy command for ctrl+c with active selection")
	}
	if updated.quitting {
		t.Fatal("expected ctrl+c with selection not to quit")
	}
	if got := updated.selection.selectedText(); got != "hello" {
		t.Fatalf("expected selection to remain, got %q", got)
	}
}

func TestReplSelectionCmdCCopiesSelection(t *testing.T) {
	m := newTestModel()
	m.output.AddLine("hello world")
	m.updateViewportContent()
	m.selection.start(0, 0, 0)
	m.selection.drag(5, 0, 0)

	updated, cmd := m.updateNormalMode(tea.KeyPressMsg{Code: 'c', Mod: tea.ModSuper})
	if cmd == nil {
		t.Fatal("expected copy command for cmd+c with active selection")
	}
	if updated.quitting {
		t.Fatal("expected cmd+c with selection not to quit")
	}
}

func TestReplInputSelectionMouseDragSelectsWithoutCopying(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("hello world")
	m.blurInput()
	textY := m.inputAreaTop() + 1

	updated, cmd := m.updateNormalMode(tea.MouseClickMsg(tea.Mouse{X: inputPromptWidth, Y: textY, Button: tea.MouseLeft}))
	if cmd == nil {
		t.Fatal("expected focus command on input mouse down")
	}
	updated, cmd = updated.updateNormalMode(tea.MouseMotionMsg(tea.Mouse{X: inputPromptWidth + 5, Y: textY, Button: tea.MouseLeft}))
	if cmd != nil {
		t.Fatal("expected no command while dragging input selection")
	}
	updated, cmd = updated.updateNormalMode(tea.MouseReleaseMsg(tea.Mouse{X: inputPromptWidth + 5, Y: textY, Button: tea.MouseLeft}))
	if cmd != nil {
		t.Fatal("expected no copy command on input mouse release")
	}
	if got := updated.inputSelection.selectedText(); got != "hello" {
		t.Fatalf("expected input selection to remain, got %q", got)
	}
	if strings.Contains(updated.selection.selectedText(), "hello") {
		t.Fatalf("expected viewport selection to stay clear, got %q", updated.selection.selectedText())
	}
	if !updated.textarea.Focused() {
		t.Fatal("expected input click to focus textarea")
	}
}

func TestReplInputSelectionCtrlCCopiesSelection(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("hello world")
	m.inputSelection.setContent(m.textarea.Value())
	m.inputSelection.start(0, 0, 0)
	m.inputSelection.drag(5, 0, 0)

	updated, cmd := m.updateNormalMode(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected copy command for ctrl+c with active input selection")
	}
	if updated.quitting {
		t.Fatal("expected ctrl+c with input selection not to quit")
	}
	if got := updated.inputSelection.selectedText(); got != "hello" {
		t.Fatalf("expected input selection to remain, got %q", got)
	}
}

func TestView_RendersInputSelection(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("hello world")
	m.inputSelection.setContent(m.textarea.Value())
	m.inputSelection.start(0, 0, 0)
	m.inputSelection.drag(5, 0, 0)

	view := m.View().Content
	plainView := ansi.Strip(view)
	if !strings.Contains(plainView, "hello") {
		t.Fatalf("expected input content in view, got %q", view)
	}
	withoutSelection := newTestModel()
	withoutSelection.textarea.SetValue("hello world")
	if view == withoutSelection.View().Content {
		t.Fatal("expected input selection to affect rendered view")
	}
}

func TestView_KeepsAltScreenMouseCaptureForSelection(t *testing.T) {
	m := newTestModel()

	view := m.View()

	if !view.AltScreen {
		t.Fatal("expected alt-screen to remain enabled")
	}
	if view.MouseMode != tea.MouseModeCellMotion {
		t.Fatalf("expected cell motion mouse capture, got %v", view.MouseMode)
	}
}
