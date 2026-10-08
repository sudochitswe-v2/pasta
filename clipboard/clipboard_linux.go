//go:build linux

package clipboard

/*
#cgo pkg-config: x11
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <pthread.h>
#include <stdlib.h>
#include <string.h>

// ---- Shared state (guarded by g_mu) ----
static Display *g_dpy = NULL;
static Window   g_win = 0;
static char    *g_text = NULL;
static size_t   g_len = 0;
static pthread_mutex_t g_mu = PTHREAD_MUTEX_INITIALIZER;

static Atom A_CLIPBOARD;
static Atom A_UTF8;
static Atom A_TARGETS;
static Atom A_TIMESTAMP;
static Atom A_INCR;

// Must be called on the same OS thread that will run the event loop.
// Returns 0 on success, -1 when no display / window creation failed.
static int pasta_clipboard_init(void) {
	Display *dpy = XOpenDisplay(NULL);
	if (!dpy) return -1;
	g_dpy = dpy;

	A_CLIPBOARD = XInternAtom(dpy, "CLIPBOARD", False);
	A_UTF8      = XInternAtom(dpy, "UTF8_STRING", False);
	A_TARGETS   = XInternAtom(dpy, "TARGETS", False);
	A_TIMESTAMP = XInternAtom(dpy, "TIMESTAMP", False);
	A_INCR      = XInternAtom(dpy, "INCR", False);

	Window root = DefaultRootWindow(dpy);
	g_win = XCreateSimpleWindow(dpy, root, 0, 0, 1, 1, 0, 0, 0);
	if (!g_win) return -1;

	XSetSelectionOwner(dpy, A_CLIPBOARD, g_win, CurrentTime);
	XFlush(dpy);
	return 0;
}

// Copy text into the shared buffer and (re-)assert selection ownership.
// Ownership must be re-asserted on every SetText because another
// application may have taken the CLIPBOARD selection since our last write.
static void pasta_clipboard_set_text(const char *s, size_t len) {
	pthread_mutex_lock(&g_mu);
	free(g_text);
	g_text = NULL;
	g_len = 0;
	if (len > 0) {
		g_text = (char*)malloc(len);
		if (g_text) {
			memcpy(g_text, s, len);
			g_len = len;
		}
	}
	pthread_mutex_unlock(&g_mu);

	if (g_dpy) {
		XSetSelectionOwner(g_dpy, A_CLIPBOARD, g_win, CurrentTime);
		XFlush(g_dpy);
	}
}

// Blocking event loop. Services SelectionRequest events so other X clients
// can read the clipboard. Blocks inside XNextEvent (no polling, 0% idle CPU).
// Returns only on fatal X error.
static int pasta_clipboard_serve(void) {
	Display *dpy = g_dpy;
	XEvent ev;
	Atom supported[4];

	while (1) {
		XNextEvent(dpy, &ev);
		if (ev.type == SelectionRequest) {
			XSelectionRequestEvent *req = &ev.xselectionrequest;
			XSelectionEvent notify;
			memset(&notify, 0, sizeof(notify));
			notify.type = SelectionNotify;
			notify.display = req->display;
			notify.requestor = req->requestor;
			notify.selection = req->selection;
			notify.target = req->target;
			notify.time = req->time;
			// Default: refuse.
			notify.property = None;

			if (req->selection == A_CLIPBOARD && req->property != None) {
				if (req->target == A_TARGETS) {
					supported[0] = A_TIMESTAMP;
					supported[1] = A_TARGETS;
					supported[2] = XA_STRING;
					supported[3] = A_UTF8;
					XChangeProperty(dpy, req->requestor, req->property,
						XA_ATOM, 32, PropModeReplace,
						(unsigned char*)supported, 4);
					notify.property = req->property;
				} else if (req->target == A_UTF8 || req->target == XA_STRING) {
					pthread_mutex_lock(&g_mu);
					if (g_text && g_len > 0) {
						XChangeProperty(dpy, req->requestor, req->property,
							req->target, 8, PropModeReplace,
							(unsigned char*)g_text, (int)g_len);
						notify.property = req->property;
					} else {
						// Empty clipboard: set zero-length property so the
						// requestor unblocks instead of hanging.
						XChangeProperty(dpy, req->requestor, req->property,
							req->target, 8, PropModeReplace,
							(unsigned char*)"", 0);
						notify.property = req->property;
					}
					pthread_mutex_unlock(&g_mu);
				} else if (req->target == A_TIMESTAMP) {
					Time t = CurrentTime;
					XChangeProperty(dpy, req->requestor, req->property,
						XA_INTEGER, 32, PropModeReplace,
						(unsigned char*)&t, 1);
					notify.property = req->property;
				}
			}
			XSendEvent(dpy, req->requestor, False, 0, (XEvent*)&notify);
			XFlush(dpy);
		} else if (ev.type == SelectionClear) {
			// Another client took the selection; nothing to do until the
			// next SetText re-asserts ownership.
		}
	}
	return 0;
}

static Window pasta_clipboard_owner(void) {
	if (!g_dpy) return 0;
	return XGetSelectionOwner(g_dpy, A_CLIPBOARD);
}

static Window pasta_clipboard_window(void) { return g_win; }
*/
import "C"

import (
	"errors"
	"os"
	"runtime"
	"sync"
	"unsafe"
)

// linuxClipboard owns the X11 CLIPBOARD selection.
type linuxClipboard struct {
	winOnce sync.Once
}

// New connects to the X server and starts the SelectionRequest event loop.
//
// If DISPLAY is unset but WAYLAND_DISPLAY is set, native Wayland clipboard
// ownership requires compositor cooperation that cannot be provided without
// a Wayland event dispatch loop; run under XWayland (which provides a
// DISPLAY) instead. New returns ErrUnavailable in that case.
func New() (Clipboard, error) {
	if os.Getenv("DISPLAY") == "" {
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			return nil, errors.New("clipboard: pure-Wayland session detected; run under XWayland (DISPLAY must be set)")
		}
		return nil, ErrUnavailable
	}
	// Xlib + the blocking serve loop must stay on one OS thread.
	runtime.LockOSThread()
	if rc := C.pasta_clipboard_init(); rc != 0 {
		runtime.UnlockOSThread()
		return nil, errors.New("clipboard: XOpenDisplay failed; is an X server running?")
	}
	c := &linuxClipboard{}
	go func() {
		// Dedicated goroutine, blocked in XNextEvent. Event-driven: 0% idle CPU.
		runtime.LockOSThread()
		C.pasta_clipboard_serve()
	}()
	return c, nil
}

// SetText implements Clipboard.
func (c *linuxClipboard) SetText(text string) error {
	if text == "" {
		return ErrEmptyText
	}
	if len(text) > MaxTextBytes {
		return errors.New("clipboard: text exceeds 64KB limit")
	}
	buf := []byte(text)
	var ptr unsafe.Pointer
	if len(buf) > 0 {
		ptr = unsafe.Pointer(&buf[0])
	}
	C.pasta_clipboard_set_text((*C.char)(ptr), C.size_t(len(buf)))
	// Verify we actually own the selection now.
	if C.pasta_clipboard_owner() != C.pasta_clipboard_window() {
		return errors.New("clipboard: failed to own X11 CLIPBOARD selection")
	}
	return nil
}
