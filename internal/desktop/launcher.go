package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// OpenDesktopWindow opens the app in a standalone, dedicated desktop window (app-mode)
// without address bar or browser controls, or falls back to system browser.
func OpenDesktopWindow(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return openDarwinApp(url)
	case "windows":
		return openWindowsApp(url)
	case "linux":
		return openLinuxApp(url)
	default:
		return exec.Command("open", url).Start()
	}
}

func appArgs(url string) []string {
	args := []string{
		fmt.Sprintf("--app=%s", url),
		"--window-size=1280,840",
		"--disable-features=Translate,OptimizationHints,OptimizationGuideModelDownloading,OptimizationHintsFetching",
		"--disable-translate",
		"--no-default-browser-check",
		"--disable-save-password-bubble",
		"--password-store=basic",
		"--no-first-run",
		"--disable-sync",
		"--disable-session-crashed-bubble",
		"--disable-infobars",
	}

	if home, err := os.UserHomeDir(); err == nil {
		profileDir := filepath.Join(home, ".rdp-app", "browser_profile")
		_ = os.MkdirAll(profileDir, 0755)
		args = append(args, fmt.Sprintf("--user-data-dir=%s", profileDir))
	}
	return args
}

func openDarwinApp(url string) error {
	candidates := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		"/Applications/Arc.app/Contents/MacOS/Arc",
	}

	args := appArgs(url)
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			cmd := exec.Command(path, args...)
			if err := cmd.Start(); err == nil {
				return nil
			}
		}
	}

	// Fallback to default browser
	return exec.Command("open", url).Start()
}

func openWindowsApp(url string) error {
	candidates := []string{
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
	}

	args := appArgs(url)
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			cmd := exec.Command(path, args...)
			if err := cmd.Start(); err == nil {
				return nil
			}
		}
	}

	// Fallback to start command
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func openLinuxApp(url string) error {
	browsers := []string{"google-chrome", "chromium-browser", "chromium", "brave-browser", "microsoft-edge"}
	args := appArgs(url)
	for _, b := range browsers {
		if path, err := exec.LookPath(b); err == nil {
			cmd := exec.Command(path, args...)
			if err := cmd.Start(); err == nil {
				return nil
			}
		}
	}

	// Fallback to xdg-open
	return exec.Command("xdg-open", url).Start()
}
