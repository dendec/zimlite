//go:build cgo && !windows

// Package textdraw loads the optional, isolated text-shaping backend.
package textdraw

/*
#cgo linux LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>
typedef unsigned char* (*render_fn)(const char*,const char*,int,int,int*,int*);
typedef void (*clear_fn)(void);
static void* backend;
static render_fn draw;
static clear_fn clear;
static int kb_load(const char* file) {
 if (backend) return 1;
 backend=dlopen(file, RTLD_NOW|RTLD_LOCAL);
 if(!backend)return 0;
 draw=(render_fn)dlsym(backend,"kb_render");clear=(clear_fn)dlsym(backend,"kb_clear");
 if(!draw||!clear){dlclose(backend);backend=NULL;return 0;}return 1;
}
static unsigned char* kb_draw(const char* font,const char* text,int size,int maxw,int* w,int* h){return draw?draw(font,text,size,maxw,w,h):NULL;}
static void kb_release(){if(clear)clear();if(backend)dlclose(backend);backend=NULL;draw=NULL;clear=NULL;}
*/
import "C"
import (
	"path/filepath"
	"unsafe"
)

func Render(root, text string, size, maxWidth int) ([]byte, int, int) {
	return RenderFont(root, filepath.Join(root, "fonts", "unifont.otf"), text, size, maxWidth)
}

func RenderFont(root, fontPath, text string, size, maxWidth int) ([]byte, int, int) {
	backend := C.CString(filepath.Join(root, "lib", "libzimlite-text.so"))
	defer C.free(unsafe.Pointer(backend))
	if C.kb_load(backend) == 0 {
		return nil, 0, 0
	}
	font := C.CString(fontPath)
	defer C.free(unsafe.Pointer(font))
	str := C.CString(text)
	defer C.free(unsafe.Pointer(str))
	var w, h C.int
	pixels := C.kb_draw(font, str, C.int(size), C.int(maxWidth), &w, &h)
	if pixels == nil {
		return nil, 0, 0
	}
	defer C.free(unsafe.Pointer(pixels))
	if w <= 0 || w > 4096 || h <= 0 || h > 96 {
		return nil, 0, 0
	}
	return C.GoBytes(unsafe.Pointer(pixels), w*h*4), int(w), int(h)
}
func Clear() { C.kb_release() }
