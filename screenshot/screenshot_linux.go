//go:build linux

package screenshot

/*
#cgo pkg-config: x11
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <stdlib.h>

static int pasta_x11_silent_error_handler(Display *d, XErrorEvent *e) {
	return 0;
}

static void pasta_x11_to_rgba(XImage *img, unsigned char *out_rgba) {
	int w = img->width;
	int h = img->height;
	unsigned long r_mask = img->red_mask;
	unsigned long g_mask = img->green_mask;
	unsigned long b_mask = img->blue_mask;

	int r_shift = 0; while ((r_mask >> r_shift) > 255) r_shift += 8;
	int g_shift = 0; while ((g_mask >> g_shift) > 255) g_shift += 8;
	int b_shift = 0; while ((b_mask >> b_shift) > 255) b_shift += 8;

	for (int y = 0; y < h; y++) {
		for (int x = 0; x < w; x++) {
			unsigned long px = XGetPixel(img, x, y);
			int idx = (y * w + x) * 4;
			out_rgba[idx + 0] = (unsigned char)((px & r_mask) >> r_shift);
			out_rgba[idx + 1] = (unsigned char)((px & g_mask) >> g_shift);
			out_rgba[idx + 2] = (unsigned char)((px & b_mask) >> b_shift);
			out_rgba[idx + 3] = 255;
		}
	}
}

static int pasta_capture_x11(int *width, int *height, unsigned char **out_buf) {
	int (*old_handler)(Display *, XErrorEvent *) = XSetErrorHandler(pasta_x11_silent_error_handler);

	Display *dpy = XOpenDisplay(NULL);
	if (!dpy) {
		XSetErrorHandler(old_handler);
		return -1;
	}

	int screen = DefaultScreen(dpy);
	Window root = RootWindow(dpy, screen);
	int w = DisplayWidth(dpy, screen);
	int h = DisplayHeight(dpy, screen);
	int depth = DefaultDepth(dpy, screen);

	unsigned long plane_mask = (1UL << depth) - 1;
	XImage *img = XGetImage(dpy, root, 0, 0, w, h, plane_mask, ZPixmap);
	XSync(dpy, False);
	XSetErrorHandler(old_handler);

	if (!img) {
		XCloseDisplay(dpy);
		return -2;
	}

	unsigned char *rgba = (unsigned char *)malloc((size_t)w * h * 4);
	if (!rgba) {
		XDestroyImage(img);
		XCloseDisplay(dpy);
		return -3;
	}

	pasta_x11_to_rgba(img, rgba);
	*width = w;
	*height = h;
	*out_buf = rgba;

	XDestroyImage(img);
	XCloseDisplay(dpy);
	return 0;
}
*/
import "C"

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"time"
	"unsafe"
)

type linuxCapturer struct{}

func newCapturer() (Capturer, error) {
	// Verify that either DISPLAY or WAYLAND_DISPLAY is set.
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return nil, ErrUnavailable
	}
	return &linuxCapturer{}, nil
}

func (c *linuxCapturer) CapturePNG() ([]byte, error) {
	// If in a Wayland session, try Wayland-native screen capture tools first.
	isWayland := os.Getenv("WAYLAND_DISPLAY") != ""
	if isWayland {
		if data, err := c.captureWayland(); err == nil {
			return data, nil
		}
	}

	// Try native X11 screen capture.
	data, err := c.captureX11()
	if err == nil {
		return data, nil
	}

	// If X11 failed (e.g. XWayland BadMatch) and we didn't try Wayland tools yet, try them now.
	if !isWayland {
		if data, err := c.captureWayland(); err == nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("screenshot capture failed: %w", err)
}

func (c *linuxCapturer) captureX11() ([]byte, error) {
	var w, h C.int
	var buf *C.uchar

	ret := C.pasta_capture_x11(&w, &h, &buf)
	if ret != 0 {
		return nil, errors.New("x11 capture failed")
	}
	defer C.free(unsafe.Pointer(buf))

	width := int(w)
	height := int(h)
	if width <= 0 || height <= 0 {
		return nil, errors.New("invalid screen dimensions from x11")
	}

	img := &image.RGBA{
		Pix:    unsafe.Slice((*byte)(buf), width*height*4),
		Stride: width * 4,
		Rect:   image.Rect(0, 0, width, height),
	}

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, fmt.Errorf("png encode: %w", err)
	}
	return out.Bytes(), nil
}

func (c *linuxCapturer) captureWayland() ([]byte, error) {
	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("pasta-shot-%d.png", time.Now().UnixNano()))
	defer os.Remove(tmpPath)

	type tool struct {
		name string
		args []string
	}

	tools := []tool{
		{"spectacle", []string{"-b", "-n", "-o", tmpPath}},
		{"grim", []string{tmpPath}},
		{"gnome-screenshot", []string{"-f", tmpPath}},
	}

	for _, t := range tools {
		if path, err := exec.LookPath(t.name); err == nil {
			cmd := exec.Command(path, t.args...)
			if err := cmd.Run(); err == nil {
				data, readErr := os.ReadFile(tmpPath)
				if readErr == nil && len(data) > 0 {
					return data, nil
				}
			}
		}
	}

	return nil, errors.New("no compatible wayland screenshot tool available")
}
