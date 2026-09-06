//go:build windows

package services

import (
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows notification via a Shell_NotifyIcon balloon. Wails does not expose
// the tray icon's window handle, so a zero-sized message-only window hosts a
// temporary tray icon: NIM_ADD, then NIM_MODIFY with NIF_INFO pops the
// balloon, and a delayed NIM_DELETE cleans the icon up. The balloon is
// display-only — no click callback, no message pump needed.
func showNotification(title, body string) {
	// CreateWindowExW and DestroyWindow must run on the same thread
	// (DestroyWindow fails cross-thread with ERROR_WINDOW_OF_OTHER_THREAD),
	// so the whole lifecycle — host window, NIM_ADD/MODIFY and the delayed
	// cleanup — is pinned to one locked OS thread. The dedicated goroutine
	// keeps the caller (the goal-notifier loop) unblocked during the cleanup
	// delay; one goroutine per notification is negligible at notify rates.
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		hwnd, ok := newNotifyHostWindow()
		if !ok {
			return
		}
		nid := notifyIconData{
			cbSize:      uint32(unsafe.Sizeof(notifyIconData{})),
			hWnd:        hwnd,
			uID:         notifyIconID,
			uFlags:      nifInfo,
			dwInfoFlags: niifInfo,
		}
		utf16Fill(nid.szInfoTitle[:], title)
		utf16Fill(nid.szInfo[:], body)

		if r, _, _ := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid))); r == 0 {
			_, _, _ = procDestroyWindow.Call(hwnd)
			return
		}
		if r, _, _ := procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid))); r == 0 {
			_, _, _ = procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
			_, _, _ = procDestroyWindow.Call(hwnd)
			return
		}
		// The balloon is now owned by the shell; retire the temp icon shortly
		// after it has been displayed (the shell picks the actual duration).
		// The sleep only parks this disposable goroutine's locked thread.
		time.Sleep(notifyCleanupDelay)
		_, _, _ = procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
		_, _, _ = procDestroyWindow.Call(hwnd)
	}()
}

var (
	procShellNotifyIconW = windows.NewLazySystemDLL("shell32.dll").NewProc("Shell_NotifyIconW")
	procCreateWindowExW  = windows.NewLazySystemDLL("user32.dll").NewProc("CreateWindowExW")
	procDestroyWindow    = windows.NewLazySystemDLL("user32.dll").NewProc("DestroyWindow")
)

// Shell_NotifyIcon message and NOTIFYICONDATA flag constants.
const (
	nimAdd    = 0
	nimModify = 1
	nimDelete = 2

	nifInfo  = 0x00000010 // szInfo/szInfoTitle/dwInfoFlags are valid
	niifInfo = 0x00000001

	// hwndMessage is (HWND)-3: a window that only receives messages.
	hwndMessage = ^uintptr(2)
	// notifyIconID distinguishes this temp icon from real tray icons.
	notifyIconID = 0x4D4B

	// notifyCleanupDelay is how long the temp icon outlives the balloon.
	notifyCleanupDelay = 12 * time.Second
)

// notifyIconData mirrors NOTIFYICONDATAW (Vista size), field order matters:
// it is passed to Shell_NotifyIconW by address.
type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         windows.GUID
	hBalloonIcon     uintptr
}

// newNotifyHostWindow creates a zero-sized message-only window using the
// system-registered "STATIC" class, so no class registration is needed.
func newNotifyHostWindow() (uintptr, bool) {
	className, _ := windows.UTF16PtrFromString("STATIC")
	hwnd, _, _ := procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(className)), 0, 0,
		0, 0, 0, 0, hwndMessage, 0, 0, 0,
	)
	return hwnd, hwnd != 0
}

// utf16Fill copies s into dst as a NUL-terminated UTF-16 string, silently
// truncating when the fixed-size buffer is too small.
func utf16Fill(dst []uint16, s string) {
	src := windows.StringToUTF16(s)
	n := min(len(src), len(dst)-1)
	copy(dst[:n], src[:n])
}
