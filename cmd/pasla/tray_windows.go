//go:build windows

package main

import (
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// A notification-area ("system tray") icon, so people can see that Pasla is
// running and have an obvious way to open or quit it. Plain Win32 through
// syscall: no cgo, no dependencies. Any failure just means no icon; Pasla
// keeps working and can be reopened by starting the exe again.

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	pRegisterClassExW    = user32.NewProc("RegisterClassExW")
	pCreateWindowExW     = user32.NewProc("CreateWindowExW")
	pDefWindowProcW      = user32.NewProc("DefWindowProcW")
	pDestroyWindow       = user32.NewProc("DestroyWindow")
	pGetMessageW         = user32.NewProc("GetMessageW")
	pTranslateMessage    = user32.NewProc("TranslateMessage")
	pDispatchMessageW    = user32.NewProc("DispatchMessageW")
	pPostMessageW        = user32.NewProc("PostMessageW")
	pPostQuitMessage     = user32.NewProc("PostQuitMessage")
	pCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	pAppendMenuW         = user32.NewProc("AppendMenuW")
	pSetMenuDefaultItem  = user32.NewProc("SetMenuDefaultItem")
	pTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	pDestroyMenu         = user32.NewProc("DestroyMenu")
	pSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	pGetCursorPos        = user32.NewProc("GetCursorPos")
	pLoadImageW          = user32.NewProc("LoadImageW")
	pGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	pShellNotifyIconW    = shell32.NewProc("Shell_NotifyIconW")
	pGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
)

const (
	wmDestroy     = 0x0002
	wmClose       = 0x0010
	wmNull        = 0x0000
	wmCommand     = 0x0111
	wmLButtonUp   = 0x0202
	wmRButtonUp   = 0x0205
	wmContextMenu = 0x007B
	wmApp         = 0x8000
	wmTray        = wmApp + 1

	nimAdd    = 0
	nimModify = 1
	nimDelete = 2
	nifMsg    = 0x1
	nifIcon   = 0x2
	nifTip    = 0x4
	nifInfo   = 0x10
	niifInfo  = 0x1

	mfString     = 0x0
	mfSeparator  = 0x800
	tpmRightBtn  = 0x0002
	tpmReturnCmd = 0x0100
	tpmNoNotify  = 0x0080

	imageIcon    = 1
	smCxSmIcon   = 49
	smCySmIcon   = 50
	cwUseDefault = 0x80000000
	wsOverlapped = 0x00000000
	idOpen       = 1
	idQuit       = 2
	appIconResID = 1 // the RT_GROUP_ICON written by tools/winres
)

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   syscall.Handle
	Icon       syscall.Handle
	Cursor     syscall.Handle
	Background syscall.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     syscall.Handle
}

type notifyIconData struct {
	Size            uint32
	Wnd             syscall.Handle
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            syscall.Handle
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GUIDItem        [16]byte
	BalloonIcon     syscall.Handle
}

type point struct{ X, Y int32 }

type winMsg struct {
	Hwnd    syscall.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
	Private uint32
}

// trayState lives for the life of the process; Win32 calls back into it.
var trayMu sync.Mutex

var tray struct {
	gone   bool
	hwnd   syscall.Handle
	nid    notifyIconData
	open   func()
	quit   func()
	labels trayLabels
}

// startTray shows the icon and returns a function that removes it.
func startTray(open, quit func(), labels trayLabels) func() {
	tray.open, tray.quit, tray.labels = open, quit, labels
	ready := make(chan bool, 1)
	go func() {
		defer func() {
			if recover() != nil {
				select {
				case ready <- false:
				default:
				}
			}
		}()
		runtime.LockOSThread() // a window's messages must be pumped by the thread that made it
		if !createTrayWindow() {
			ready <- false
			return
		}
		ready <- true
		var m winMsg
		for {
			r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}()
	if !<-ready {
		return func() {}
	}
	return func() {
		// Remove the icon right here: the process exits right after this,
		// before the tray thread would get to it, and Windows would leave a
		// dead icon behind until the mouse passes over it.
		trayMu.Lock()
		pShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&tray.nid)))
		tray.gone = true
		trayMu.Unlock()
		pPostMessageW.Call(uintptr(tray.hwnd), wmClose, 0, 0)
	}
}

// trayNotify shows a short balloon (a notification on Windows 10/11) from
// the tray icon. It does nothing when there is no icon.
func trayNotify(title, text string) {
	trayMu.Lock()
	defer trayMu.Unlock()
	if tray.hwnd == 0 || tray.gone {
		return
	}
	nid := tray.nid
	nid.Flags = nifInfo
	nid.InfoFlags = niifInfo
	t, _ := syscall.UTF16FromString(title)
	copy(nid.InfoTitle[:len(nid.InfoTitle)-1], t)
	b, _ := syscall.UTF16FromString(text)
	copy(nid.Info[:len(nid.Info)-1], b)
	pShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

func createTrayWindow() bool {
	inst, _, _ := pGetModuleHandleW.Call(0)
	class, _ := syscall.UTF16PtrFromString("PaslaTrayWindow")
	wc := wndClassEx{
		WndProc:   syscall.NewCallback(trayWndProc),
		Instance:  syscall.Handle(inst),
		ClassName: class,
	}
	wc.Size = uint32(unsafe.Sizeof(wc))
	if r, _, _ := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return false
	}
	title, _ := syscall.UTF16PtrFromString("Pasla")
	// A hidden top-level window: it is never shown, it only receives the
	// icon's messages and owns the context menu.
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)),
		wsOverlapped, cwUseDefault, cwUseDefault, cwUseDefault, cwUseDefault, 0, 0, inst, 0)
	if hwnd == 0 {
		return false
	}
	tray.hwnd = syscall.Handle(hwnd)

	cx, _, _ := pGetSystemMetrics.Call(smCxSmIcon)
	cy, _, _ := pGetSystemMetrics.Call(smCySmIcon)
	icon, _, _ := pLoadImageW.Call(inst, appIconResID, imageIcon, cx, cy, 0)

	tray.nid.Size = uint32(unsafe.Sizeof(tray.nid))
	tray.nid.Wnd = tray.hwnd
	tray.nid.ID = 1
	tray.nid.Flags = nifMsg | nifIcon | nifTip
	tray.nid.CallbackMessage = wmTray
	tray.nid.Icon = syscall.Handle(icon)
	tip, _ := syscall.UTF16FromString(tray.labels.Tooltip)
	copy(tray.nid.Tip[:len(tray.nid.Tip)-1], tip)
	if r, _, _ := pShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&tray.nid))); r == 0 {
		pDestroyWindow.Call(hwnd)
		return false
	}
	return true
}

func trayWndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	switch msg {
	case wmTray:
		switch lparam & 0xFFFF {
		case wmLButtonUp:
			go tray.open()
		case wmRButtonUp, wmContextMenu:
			showTrayMenu(hwnd)
		}
		return 0
	case wmClose:
		trayMu.Lock()
		if !tray.gone {
			pShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&tray.nid)))
			tray.gone = true
		}
		trayMu.Unlock()
		pDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return r
}

func showTrayMenu(hwnd uintptr) {
	menu, _, _ := pCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer pDestroyMenu.Call(menu)
	open, _ := syscall.UTF16PtrFromString(tray.labels.Open)
	quit, _ := syscall.UTF16PtrFromString(tray.labels.Quit)
	pAppendMenuW.Call(menu, mfString, idOpen, uintptr(unsafe.Pointer(open)))
	pAppendMenuW.Call(menu, mfSeparator, 0, 0)
	pAppendMenuW.Call(menu, mfString, idQuit, uintptr(unsafe.Pointer(quit)))
	pSetMenuDefaultItem.Call(menu, idOpen, 0)

	var pt point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	// Without this the menu does not close when the user clicks elsewhere.
	pSetForegroundWindow.Call(hwnd)
	cmd, _, _ := pTrackPopupMenu.Call(menu, tpmRightBtn|tpmReturnCmd|tpmNoNotify,
		uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)
	pPostMessageW.Call(hwnd, wmNull, 0, 0)
	switch cmd {
	case idOpen:
		go tray.open()
	case idQuit:
		go tray.quit()
	}
}
