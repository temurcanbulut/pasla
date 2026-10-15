package node

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// OpenBrowser opens a URL with the desktop's default browser.
func OpenBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

// reveal shows a file or folder in the system file manager.
func reveal(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		path = filepath.Dir(path)
		if st, err = os.Stat(path); err != nil {
			return err
		}
	}
	switch runtime.GOOS {
	case "windows":
		if !st.IsDir() {
			return exec.Command("explorer", "/select,", path).Start()
		}
		return exec.Command("explorer", path).Start()
	case "darwin":
		if !st.IsDir() {
			return exec.Command("open", "-R", path).Start()
		}
		return exec.Command("open", path).Start()
	default:
		if !st.IsDir() {
			path = filepath.Dir(path)
		}
		return exec.Command("xdg-open", path).Start()
	}
}
