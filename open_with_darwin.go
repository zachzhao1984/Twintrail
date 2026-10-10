package main

import (
	"fmt"
	"image"
	"runtime"
	"sort"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/cstrings"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo/ui"
)

// NSWorkspace and CoreGraphics are called in-process; no script, image
// serialization, or child process is involved in opening the picker.
var applicationNative struct {
	once              sync.Once
	err               error
	colorSpace        func() uintptr
	releaseColorSpace func(uintptr)
	bitmapContext     func(unsafe.Pointer, uintptr, uintptr, uintptr, uintptr, uintptr, uint32) uintptr
	releaseContext    func(uintptr)
	drawImage         func(uintptr, applicationRect, uintptr)
}

type applicationRect struct{ X, Y, W, H float64 }

func initApplicationNative() error {
	applicationNative.once.Do(func() {
		_, applicationNative.err = purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if applicationNative.err != nil {
			return
		}
		cg, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			applicationNative.err = err
			return
		}
		purego.RegisterLibFunc(&applicationNative.colorSpace, cg, "CGColorSpaceCreateDeviceRGB")
		purego.RegisterLibFunc(&applicationNative.releaseColorSpace, cg, "CGColorSpaceRelease")
		purego.RegisterLibFunc(&applicationNative.bitmapContext, cg, "CGBitmapContextCreate")
		purego.RegisterLibFunc(&applicationNative.releaseContext, cg, "CGContextRelease")
		purego.RegisterLibFunc(&applicationNative.drawImage, cg, "CGContextDrawImage")
	})
	return applicationNative.err
}
func applicationSend(object objc.ID, selector string, args ...any) objc.ID {
	return object.Send(objc.RegisterName(selector), args...)
}
func applicationString(s string) objc.ID {
	return applicationSend(objc.ID(objc.GetClass("NSString")), "stringWithUTF8String:", s)
}

// Keep the autorelease pool on the OS thread where the objects were created.
func applicationPool() func() {
	runtime.LockOSThread()
	pool := applicationSend(objc.ID(objc.GetClass("NSAutoreleasePool")), "new")
	return func() { applicationSend(pool, "drain"); runtime.UnlockOSThread() }
}

func systemOpenWithApplications(paths []string) ([]openWithApplication, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	if err := initApplicationNative(); err != nil {
		return nil, err
	}
	drain := applicationPool()
	defer drain()
	workspace := applicationSend(objc.ID(objc.GetClass("NSWorkspace")), "sharedWorkspace")
	manager := applicationSend(objc.ID(objc.GetClass("NSFileManager")), "defaultManager")
	var applications []openWithApplication
	for i, path := range paths {
		url := applicationSend(objc.ID(objc.GetClass("NSURL")), "fileURLWithPath:", applicationString(path))
		handlers := applicationSend(workspace, "URLsForApplicationsToOpenURL:", url)
		defaultURL := applicationSend(workspace, "URLForApplicationToOpenURL:", url)
		defaultPath := cstrings.NSStringToString(applicationSend(defaultURL, "path"))
		available := make(map[string]bool)
		count := int(applicationSend(handlers, "count"))
		for j := 0; j < count; j++ {
			handler := applicationSend(handlers, "objectAtIndex:", uintptr(j))
			handlerPath := cstrings.NSStringToString(applicationSend(handler, "path"))
			available[handlerPath] = true
			if i == 0 {
				name := cstrings.NSStringToString(applicationSend(manager, "displayNameAtPath:", applicationString(handlerPath)))
				applications = append(applications, openWithApplication{Name: strings.TrimSuffix(name, ".app"), Path: handlerPath, Default: handlerPath == defaultPath})
			}
		}
		if i > 0 {
			filtered := applications[:0]
			for _, application := range applications {
				if available[application.Path] {
					application.Default = application.Default && application.Path == defaultPath
					filtered = append(filtered, application)
				}
			}
			applications = filtered
		}
	}
	sort.Slice(applications, func(i, j int) bool {
		if applications[i].Default != applications[j].Default {
			return applications[i].Default
		}
		return strings.ToLower(applications[i].Name) < strings.ToLower(applications[j].Name)
	})
	if len(applications) == 0 || !applications[0].Default {
		applications = append([]openWithApplication{{Name: "Default Application", Default: true}}, applications...)
	}
	return applications, nil
}

func systemApplicationIcon(path string) (*ui.Bitmap, error) {
	if path == "" {
		return nil, nil
	}
	if err := initApplicationNative(); err != nil {
		return nil, err
	}
	drain := applicationPool()
	defer drain()
	workspace := applicationSend(objc.ID(objc.GetClass("NSWorkspace")), "sharedWorkspace")
	nsImage := applicationSend(workspace, "iconForFile:", applicationString(path))
	return applicationBitmap(nsImage)
}

func applicationBitmap(nsImage objc.ID) (*ui.Bitmap, error) {
	// Render to the destination's pixel dimensions, rather than changing
	// NSImage.size (which leaves its 1024px TIFF representations intact).
	cgImage := applicationSend(nsImage, "CGImageForProposedRect:context:hints:", uintptr(0), objc.ID(0), objc.ID(0))
	if cgImage == 0 {
		return nil, fmt.Errorf("application icon unavailable")
	}
	const size = 64
	bitmap := image.NewRGBA(image.Rect(0, 0, size, size))
	colorSpace := applicationNative.colorSpace()
	defer applicationNative.releaseColorSpace(colorSpace)
	context := applicationNative.bitmapContext(unsafe.Pointer(&bitmap.Pix[0]), size, size, 8, size*4, colorSpace, 1) // premultiplied RGBA
	if context == 0 {
		return nil, fmt.Errorf("application icon bitmap unavailable")
	}
	applicationNative.drawImage(context, applicationRect{W: size, H: size}, uintptr(cgImage))
	applicationNative.releaseContext(context)
	runtime.KeepAlive(bitmap)
	return ui.NewBitmap(bitmap), nil
}
