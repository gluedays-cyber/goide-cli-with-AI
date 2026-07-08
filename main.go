package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	app         *tview.Application
	editor      *tview.TextArea
	promptInput *tview.TextArea
	statusBar   *tview.TextView
	separator   *tview.TextView
	mainFlex    *tview.Flex

	// Path of the temporary file currently being edited
	tempFilePath string

	// API Keys from AIConfig
	googleKey  string
	groqKey    string
	ollamaKey  string
	upstageKey string

	// Currently selected model and list of available models
	selectedModel   ModelOption
	availableModels []ModelOption

	// Command line history (promptInput)
	promptHistory    []string
	promptHistoryIdx int    // -1 means currently typing
	promptDraft      string // Temporary save before navigating history

	// === User-defined color settings (modify these values in the source code to apply) ===
	ThemeBgColor        = "#000057ff" // Background color
	ThemeTextColor      = "#ffffff"   // Text color
	ThemeBorderColor    = "#ffff00"   // Border and title color
	SelectedTextBgColor = "#0000cd"
)

func init() {
	// Apply the colors specified by the user in the source code to the TUI theme style
	tview.Styles.PrimitiveBackgroundColor = tcell.GetColor(ThemeBgColor)
	tview.Styles.ContrastBackgroundColor = tcell.GetColor(ThemeBgColor)
	tview.Styles.MoreContrastBackgroundColor = tcell.GetColor(ThemeBgColor)
	tview.Styles.PrimaryTextColor = tcell.GetColor(ThemeTextColor)
	tview.Styles.TitleColor = tcell.GetColor(ThemeBorderColor)
	tview.Styles.BorderColor = tcell.GetColor(ThemeBorderColor)
}

// randomName returns an n-character random lowercase string.
func randomName(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// createTempFile creates an empty temporary .go file with a random name in the current directory.
func createTempFile() string {
	name := randomName(8) + ".go"
	_ = os.WriteFile(name, []byte{}, 0644)
	return name
}

// updatePromptTitle redraws the separator to update the model name.
func updatePromptTitle() {
	if separator != nil {
		// SetText is for tview rendering trigger; actual drawing is handled in SetDrawFunc.
		separator.SetText("")
	}
}

// showModelSelector opens the model selection modal with the F12 shortcut.
func showModelSelector() {
	if len(availableModels) == 0 {
		return
	}

	list := tview.NewList()
	list.SetBorder(true).SetTitle(" Select Model (Enter=Select, Esc=Cancel) ")
	list.SetSelectedBackgroundColor(tcell.GetColor("#0000b3ff"))
	list.SetSelectedTextColor(tcell.ColorWhite)

	for i, m := range availableModels {
		model := m // capture
		mark := "  "
		if model.Name == selectedModel.Name && model.Provider == selectedModel.Provider {
			mark = "▶ "
		}
		var shortcut rune
		if i < 9 {
			shortcut = rune('1' + i)
		} else {
			shortcut = 0
		}
		list.AddItem(mark+model.Alias, model.Provider, shortcut, func() {
			selectedModel = model
			updatePromptTitle()
			app.SetRoot(mainFlex, true)
			app.SetFocus(promptInput)
		})
	}

	list.SetDoneFunc(func() {
		// Close with Esc
		app.SetRoot(mainFlex, true)
		app.SetFocus(promptInput)
	})

	app.SetRoot(list, true)
}

// showInputModal opens a single input modal and calls onConfirm upon confirmation.
func showInputModal(title string, onConfirm func(input string)) {
	inputField := tview.NewInputField()
	inputField.SetLabel(" File Name: ")
	inputField.SetBorder(true).SetTitle(" " + title + " (Enter=Confirm, Esc=Cancel) ")

	inputField.SetDoneFunc(func(key tcell.Key) {
		text := strings.TrimSpace(inputField.GetText())
		app.SetRoot(mainFlex, true)
		app.SetFocus(editor)
		if key == tcell.KeyEnter && text != "" {
			onConfirm(text)
		}
	})

	app.SetRoot(inputField, true)
	app.SetFocus(inputField)
}

// saveAsGo is the F2 shortcut: takes a file name and saves it as a .go file.
func saveAsGo() {
	showInputModal("F2: Save as .go file", func(name string) {
		// Force .go extension
		if !strings.HasSuffix(name, ".go") {
			name = name + ".go"
		}
		code := editor.GetText()
		if err := os.WriteFile(name, []byte(code), 0644); err != nil {
			statusBar.SetText(fmt.Sprintf(" [black:red]Save Failed: %s[white:blue] ", err.Error()))
		} else {
			statusBar.SetText(fmt.Sprintf(" [black:green]Save Completed: %s[white:blue] ", name))
		}
	})
}

// buildExe is the F3 shortcut: takes a file name and builds the temp file to a .exe with that name.
func buildExe() {
	showInputModal("F3: Build to .exe", func(name string) {
		// Force .exe extension
		if !strings.HasSuffix(name, ".exe") {
			name = name + ".exe"
		}
		outPath, _ := filepath.Abs(name)
		BuildExe(tempFilePath, outPath)
	})
}

func main() {
	// Runs go mod init goide when the app starts. (If go.mod already exists, an error occurs but is ignored)
	_ = exec.Command("go", "mod", "init", "goide").Run()

	// Force truecolor (24-bit color) rendering
	_ = os.Setenv("COLORTERM", "truecolor")

	app = tview.NewApplication()

	// Load API Keys and model list from the configuration file
	config, options, err := LoadAIConfig()
	if err == nil {
		googleKey = config.Google.APIKey
		groqKey = config.Groq.APIKey
		ollamaKey = config.Ollama.APIKey
		upstageKey = config.Upstage.APIKey

		availableModels = options
		if len(availableModels) > 0 {
			selectedModel = availableModels[0]
		}
	}

	// Create a temporary file with a random name
	tempFilePath = createTempFile()

	// Delete temp file and exit upon receiving console X button or SIGTERM
	// Windows: CTRL_CLOSE_EVENT -> os.Interrupt, SIGTERM handles external process termination signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		_ = os.Remove(tempFilePath)
		app.Stop()
		os.Exit(0)
	}()

	// Create editor
	editor = tview.NewTextArea()
	editor.SetBorder(false)
	editor.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.GetColor(SelectedTextBgColor)).
		Foreground(tcell.GetColor(ThemeTextColor)))

	// Initial editor state: empty screen
	editor.SetText("", false)

	// Auto-save to temp file on modification
	editor.SetChangedFunc(func() {
		code := editor.GetText()
		_ = os.WriteFile(tempFilePath, []byte(code), 0644)
	})

	// System clipboard integration
	editor.SetClipboard(func(text string) {
		_ = clipboard.WriteAll(text)
	}, func() string {
		text, _ := clipboard.ReadAll()
		return text
	})

	// Override Ctrl+C key binding
	editor.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			if editor.HasSelection() {
				text, start, _ := editor.GetSelection()
				_ = clipboard.WriteAll(text)
				editor.Select(start, start)
			}
			return nil
		}
		return event
	})

	// Create separator (top line role, centered model name HR)
	separator = tview.NewTextView().SetDynamicColors(false)
	separator.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		title := selectedModel.Alias
		if title == "" {
			title = "No Model (F12)"
		}
		label := []rune(" " + title + " ")
		labelLen := len(label)
		sideLen := (width - labelLen) / 2
		if sideLen < 0 {
			sideLen = 0
		}
		rightLen := width - labelLen - sideLen
		if rightLen < 0 {
			rightLen = 0
		}
		style := tcell.StyleDefault.
			Foreground(tcell.GetColor(ThemeBorderColor)).
			Background(tcell.GetColor(ThemeBgColor))
		col := x
		for range sideLen {
			screen.SetContent(col, y, '─', nil, style)
			col++
		}
		for _, ch := range label {
			screen.SetContent(col, y, ch, nil, style)
			col++
		}
		for range rightLen {
			screen.SetContent(col, y, '─', nil, style)
			col++
		}
		return x, y, width, height
	})

	// Create AI instruction input field
	promptInput = tview.NewTextArea()
	promptInput.SetBorder(false)
	updatePromptTitle()
	promptInput.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.GetColor(SelectedTextBgColor)).
		Foreground(tcell.GetColor(ThemeTextColor)))

	// Handle Enter / Up / Down keys: AI request and history navigation
	promptHistoryIdx = -1
	promptInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			if len(promptHistory) == 0 {
				return nil
			}
			if promptHistoryIdx == -1 {
				// Temporarily save current input and move to the most recent item
				promptDraft = promptInput.GetText()
				promptHistoryIdx = len(promptHistory) - 1
			} else if promptHistoryIdx > 0 {
				promptHistoryIdx--
			}
			promptInput.SetText(promptHistory[promptHistoryIdx], false)
			return nil
		case tcell.KeyDown:
			if promptHistoryIdx == -1 {
				return nil
			}
			if promptHistoryIdx < len(promptHistory)-1 {
				promptHistoryIdx++
				promptInput.SetText(promptHistory[promptHistoryIdx], false)
			} else {
				// End of history: restore temporarily saved draft
				promptHistoryIdx = -1
				promptInput.SetText(promptDraft, false)
			}
			return nil
		case tcell.KeyEnter:
			instruction := strings.TrimSpace(promptInput.GetText())
			if instruction == "" {
				return nil
			}
			// Save history (prevent consecutive duplicates)
			if len(promptHistory) == 0 || promptHistory[len(promptHistory)-1] != instruction {
				promptHistory = append(promptHistory, instruction)
			}
			promptHistoryIdx = -1
			promptDraft = ""
			currentCode := editor.GetText()

			go func() {
				app.QueueUpdateDraw(func() {
					statusBar.SetText(" [black:yellow]AI is processing your request... Please wait.[white:blue] ")
				})

				newCode, aiErr := AskAI(currentCode, instruction)

				app.QueueUpdateDraw(func() {
					if aiErr != nil {
						statusBar.SetText(fmt.Sprintf(" [black:red]AI Error: %s[white:blue] ", aiErr.Error()))
						app.SetFocus(promptInput)
					} else {
						editor.SetText(newCode, false)
						promptInput.SetText("", false)
						statusBar.SetText(" <F2=Save> <F3=Build> <F5=Run> <F6=Switch> <F9=Shell> <F10=Exit> <F12=Model> ")
						app.SetFocus(editor)
					}
				})
			}()
			return nil
		}
		return event
	})

	// Create status bar
	statusBar = tview.NewTextView().
		SetDynamicColors(true).
		SetText(" <F2=Save> <F3=Build> <F5=Run> <F6=Switch> <F9=Shell> <F10=Exit> <F12=Model> ")

	// Configure main layout
	mainFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(editor, 0, 3, false).
		AddItem(separator, 1, 0, false).
		AddItem(promptInput, 3, 1, true).
		AddItem(statusBar, 1, 0, false)

	// Register global shortcuts
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			return nil // Prevent forced app termination
		}

		switch event.Key() {
		case tcell.KeyF2:
			saveAsGo()
			return nil
		case tcell.KeyF3:
			buildExe()
			return nil
		case tcell.KeyF5:
			RunCode(tempFilePath)
			return nil
		case tcell.KeyF6:
			if app.GetFocus() == editor {
				app.SetFocus(promptInput)
			} else {
				app.SetFocus(editor)
			}
			return nil
		case tcell.KeyF9:
			shellMode()
			return nil
		case tcell.KeyF10:
			// Delete temporary file upon app termination
			_ = os.Remove(tempFilePath)
			app.Stop()
			return nil
		case tcell.KeyF12:
			showModelSelector()
			return nil
		}
		return event
	})

	if err := app.SetRoot(mainFlex, true).Run(); err != nil {
		fmt.Printf("Error running application: %v\n", err)
		os.Exit(1)
	}
}
