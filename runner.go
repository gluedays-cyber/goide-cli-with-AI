package main

import (
	"fmt"
	"os"
	"os/exec"
)

// shellMode is the F9 shortcut: suspends the TUI and switches to PowerShell.
// Displays a fixed hint "Press F9 to return to GoIDE" on the first line of the terminal,
// and protects the first line by restricting the scroll area to the second line and below.
// Injects F9 -> exit key binding into PowerShell via PSReadLine.
func shellMode() {
	app.Suspend(func() {
		// Clear screen
		fmt.Fprint(os.Stdout, "\033[2J\033[H")

		// Line 1: Yellow background, black text fixed hint, fill color to end of line
		fmt.Fprint(os.Stdout,
			"\033[43;30m  ▶  Press F9 to return to GoIDE  \033[K\033[0m")

		// Restrict scroll area to line 2~end -> fix line 1
		fmt.Fprint(os.Stdout, "\033[2;999r")

		// Move cursor to row 2, column 1
		fmt.Fprint(os.Stdout, "\033[2;1H")

		// PSReadLine: Press F9 to enter 'exit' and press Enter
		psInit := `Set-PSReadLineKeyHandler -Key F9 -ScriptBlock {` +
			`[Microsoft.PowerShell.PSConsoleReadLine]::RevertLine();` +
			`[Microsoft.PowerShell.PSConsoleReadLine]::Insert('exit');` +
			`[Microsoft.PowerShell.PSConsoleReadLine]::AcceptLine()}`

		cmd := exec.Command("powershell", "-NoLogo", "-NoExit", "-Command", psInit)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()

		// Restore entire scroll area and clear screen
		fmt.Fprint(os.Stdout, "\033[r\033[2J\033[H")
	})
}

// RunCode executes the specified temporary .go file with go run and displays it directly on the terminal standard output.
func RunCode(filePath string) {
	app.Suspend(func() {
		fmt.Print("\033[H\033[2J") // Clear terminal screen
		fmt.Printf("--- Executing Program (%s) ---\n", filePath)

		// Run go mod tidy before executing the code.
		_ = exec.Command("go", "mod", "tidy").Run()

		cmd := exec.Command("go", "run", filePath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("\n[Execution Error]: %v\n", err)
		}

		fmt.Println("\n-------------------------")
		fmt.Print("Press Enter to return to IDE...")
		var placeholder string
		_, _ = fmt.Scanln(&placeholder)
	})
}

// BuildExe compiles srcFile and builds it to outPath (.exe).
func BuildExe(srcFile string, outPath string) {
	app.Suspend(func() {
		fmt.Print("\033[H\033[2J")
		fmt.Printf("--- Building: %s → %s ---\n", srcFile, outPath)

		cmd := exec.Command("go", "build", "-o", outPath, srcFile)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("\n[Build Error]: %v\n", err)
		} else {
			fmt.Printf("\nBuild completed: %s\n", outPath)
		}

		fmt.Println("\n-------------------------")
		fmt.Print("Press Enter to return to IDE...")
		var placeholder string
		_, _ = fmt.Scanln(&placeholder)
	})
}
