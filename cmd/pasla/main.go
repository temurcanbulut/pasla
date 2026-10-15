// Command pasla shares files between computers and phones on the same local
// network, with no internet connection, account or cloud involved.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/temurcanbulut/pasla/internal/i18n"
	"github.com/temurcanbulut/pasla/internal/node"
)

var version = "dev"

// trayLabels are the texts of the Windows notification-area icon.
type trayLabels struct{ Tooltip, Open, Quit string }

// t translates a line for the terminal, in the language chosen in the app.
func t(format string, args ...any) string { return i18n.T(i18n.Default(), format, args...) }

const usage = `Pasla — offline file and text transfer on your local network

Usage:
  pasla [flags]                      start Pasla and open the interface in the browser
  pasla send -to DEVICE PATH...      send files or folders through the running Pasla
  pasla version                      print the version

The interface language (English or Turkish) is chosen in Settings.

Flags:
`

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "send":
			os.Exit(cmdSend(os.Args[2:]))
		case "version", "-version", "--version":
			fmt.Println("pasla", version)
			return
		}
	}
	os.Exit(cmdServe(os.Args[1:]))
}

func cmdServe(args []string) int {
	fs := flag.NewFlagSet("pasla", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), usage)
		fs.PrintDefaults()
	}
	cfg := node.Config{Version: version}
	fs.StringVar(&cfg.Dir, "config", "", "settings folder (default: your user config folder/pasla)")
	fs.IntVar(&cfg.PeerPort, "port", node.DefaultPeerPort, "transfer port between devices (TCP)")
	fs.IntVar(&cfg.DiscoveryPort, "discovery-port", node.DefaultDiscoveryPort, "automatic discovery port (UDP)")
	fs.IntVar(&cfg.PhonePort, "phone-port", node.DefaultPhonePort, "phone connection port (TCP, open only while a phone connects)")
	fs.StringVar(&cfg.UIAddr, "ui", node.DefaultUIAddr, "interface address (this computer only)")
	fs.BoolVar(&cfg.NoDiscovery, "no-discovery", false, "turn off automatic discovery; add devices by IP")
	fs.StringVar(&cfg.Name, "name", "", "the name other devices see")
	fs.StringVar(&cfg.ReceiveDir, "receive-dir", "", "folder for incoming files")
	noBrowser := fs.Bool("no-browser", false, "do not open the browser at start")
	_ = fs.Parse(args)

	logger := log.New(os.Stderr, "", log.Ltime)
	url := "http://" + cfg.UIAddr

	if cfg.Dir == "" {
		dir, err := node.DefaultDir()
		if err != nil {
			logger.Print(t("could not find the settings folder: %v", err))
			alert(t("could not find the settings folder: %v", err))
			return 1
		}
		cfg.Dir = dir
	}
	i18n.SetDefault(node.ReadLang(cfg.Dir))
	if !hasConsole {
		// Started by double-click: keep a log next to the settings instead.
		if f := openLog(cfg.Dir); f != nil {
			defer f.Close()
			logger = log.New(f, "", log.Ldate|log.Ltime)
		}
	}
	cfg.Logger = logger
	cfg.Notify = trayNotify

	ln, err := node.ListenUI(cfg.UIAddr)
	if errors.Is(err, node.ErrAlreadyRunning) {
		logger.Print(t("Pasla is already running; opening the interface."))
		if *noBrowser {
			return 0
		}
		launch, err := node.RunningLaunchURL(cfg.UIAddr, cfg.Dir)
		if err != nil {
			logger.Print(t("could not reach the running Pasla: %v", err))
			alert(t("could not reach the running Pasla: %v", err))
			return 1
		}
		_ = node.OpenBrowser(launch)
		return 0
	}
	if err != nil {
		logger.Print(t("could not start the interface: %v", err))
		alert(t("could not start the interface: %v", err))
		return 1
	}

	n, err := node.New(cfg)
	if err != nil {
		logger.Print(t("could not start: %v", err))
		alert(t("could not start: %v", err))
		return 1
	}
	i18n.SetDefault(n.Store().Settings().Lang)
	s := n.Store().Settings()
	fp := n.Fingerprint()
	logger.Print(t("Pasla %s is ready", version))
	logger.Print(t("  Device     : %s  (identity %s…)", s.Name, fp[:12]))
	logger.Print(t("  Interface  : %s (for this user only)", url))
	logger.Print(t("  Received   : %s", s.ReceiveDir))
	logger.Print(t("  Ports      : TCP %v (transfers), UDP %v (discovery), TCP %v (phones, when needed)", cfg.PeerPort, cfg.DiscoveryPort, cfg.PhonePort))
	if hasConsole {
		logger.Print(t("Press Ctrl+C to quit."))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if !*noBrowser {
		go func() {
			time.Sleep(300 * time.Millisecond)
			if err := node.OpenBrowser(n.LaunchURL()); err != nil {
				logger.Print(t("could not open the browser. Run Pasla again to open the interface."))
			}
		}()
	}
	removeTray := startTray(
		func() { _ = node.OpenBrowser(n.LaunchURL()) },
		n.Quit,
		trayLabels{Tooltip: t("Pasla is running"), Open: t("Open Pasla"), Quit: t("Quit Pasla")},
	)
	defer removeTray()
	if err := n.Run(ctx, ln); err != nil {
		logger.Print(t("error: %v", err))
		alert(t("Pasla stopped because of an error: %v", err))
		return 1
	}
	logger.Print(t("Bye."))
	return 0
}

// openLog opens pasla.log in dir, starting afresh once it passes 1 MB.
func openLog(dir string) *os.File {
	path := filepath.Join(dir, "pasla.log")
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if st, err := os.Stat(path); err == nil && st.Size() > 1<<20 {
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}
	_ = os.MkdirAll(dir, 0o700)
	f, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return nil
	}
	return f
}

func cmdSend(args []string) int {
	fs := flag.NewFlagSet("pasla send", flag.ExitOnError)
	to := fs.String("to", "", "name of the paired device, or the first characters of its fingerprint (required)")
	ui := fs.String("ui", node.DefaultUIAddr, "interface address of the running Pasla")
	dir := fs.String("config", "", "settings folder of the running Pasla (default: your user config folder/pasla)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: pasla send -to DEVICE file-or-folder...")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)
	if *to == "" || fs.NArg() == 0 {
		fs.Usage()
		return 2
	}
	paths := make([]string, 0, fs.NArg())
	for _, p := range fs.Args() {
		abs, err := filepath.Abs(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		paths = append(paths, abs)
	}
	if *dir == "" {
		d, err := node.DefaultDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		*dir = d
	}
	lang := node.ReadLang(*dir)
	i18n.SetDefault(lang)
	token, err := node.ReadToken(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, i18n.Text(lang, err))
		return 1
	}
	base := "http://" + *ui
	var tv struct {
		ID        string  `json:"id"`
		PeerName  string  `json:"peerName"`
		Count     int     `json:"count"`
		Total     int64   `json:"total"`
		Done      int64   `json:"done"`
		FilesDone int     `json:"filesDone"`
		State     string  `json:"state"`
		Error     string  `json:"error"`
		Speed     float64 `json:"speed"`
	}
	call := func(method, url string, in, out any) error { return apiCall(token, lang, method, url, in, out) }
	if err := call(http.MethodPost, base+"/api/send-paths", map[string]any{"to": *to, "paths": paths}, &tv); err != nil {
		fmt.Fprintln(os.Stderr, t("could not send: %v", err))
		return 1
	}
	fmt.Println(t("Sending %v file(s) (%s) to %s…", tv.Count, human(tv.Total), tv.PeerName))
	lastState := ""
	for {
		if tv.State != lastState && tv.State == "pending" {
			fmt.Println(t("Waiting for the other side to accept…"))
		}
		lastState = tv.State
		switch tv.State {
		case "active":
			pct := 100.0
			if tv.Total > 0 {
				pct = float64(tv.Done) * 100 / float64(tv.Total)
			}
			fmt.Printf("\r  %5.1f%%  %s / %s  %s/s  (%s)   ", pct, human(tv.Done), human(tv.Total), human(int64(tv.Speed)),
				t("%v/%v files", tv.FilesDone, tv.Count))
		case "done":
			fmt.Printf("\r  %s%s\n", t("Done: %v file(s), %s.", tv.Count, human(tv.Total)), strings.Repeat(" ", 30))
			return 0
		case "rejected", "cancelled", "failed":
			fmt.Printf("\n  %s\n", t("The transfer did not finish (%s): %s", tv.State, tv.Error))
			return 1
		}
		time.Sleep(500 * time.Millisecond)
		if err := call(http.MethodGet, base+"/api/transfers/"+tv.ID, nil, &tv); err != nil {
			fmt.Fprintln(os.Stderr, "\n"+t("could not reach Pasla: %v", err))
			return 1
		}
	}
}

func apiCall(token, lang, method, url string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(node.TokenHeader, token)
	req.Header.Set(node.LangHeader, lang)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.New(t("is Pasla running? (%v)", err))
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error == "" {
			e.Error = resp.Status
		}
		return errors.New(e.Error)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func human(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
