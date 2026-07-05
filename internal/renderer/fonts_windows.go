//go:build windows

package renderer

/*
#cgo LDFLAGS: -lSDL2_ttf
#include <windows.h>
#include <SDL2/SDL_ttf.h>

typedef struct {
	const char *path;
	int size;
	TTF_Font *result;
	HANDLE done;
} FontLoadArgs;

static DWORD WINAPI loadFontThread(LPVOID arg) {
	FontLoadArgs *args = (FontLoadArgs*)arg;
	args->result = TTF_OpenFont(args->path, args->size);
	SetEvent(args->done);
	return 0;
}

static TTF_Font* openFontOnBigStack(const char *path, int size) {
	FontLoadArgs args;
	args.path = path;
	args.size = size;
	args.result = NULL;
	args.done = CreateEvent(NULL, FALSE, FALSE, NULL);
	if (!args.done) return NULL;

	// Use STACK_SIZE_PARAM_IS_A_RESERVATION (0x00010000) to ensure the stack size is reserved.
	HANDLE thread = CreateThread(NULL, 64*1024*1024, loadFontThread, &args, 0x00010000, NULL);
	if (!thread) {
		CloseHandle(args.done);
		return NULL;
	}
	WaitForSingleObject(args.done, 30000);
	CloseHandle(args.done);
	CloseHandle(thread);
	return args.result;
}

typedef struct {
	SDL_RWops *rw;
	int freerw;
	int size;
	TTF_Font *result;
	HANDLE done;
} FontLoadRWArgs;

static DWORD WINAPI loadFontRWThread(LPVOID arg) {
	FontLoadRWArgs *args = (FontLoadRWArgs*)arg;
	args->result = TTF_OpenFontRW(args->rw, args->freerw, args->size);
	SetEvent(args->done);
	return 0;
}

static TTF_Font* openFontRWOnBigStack(SDL_RWops *rw, int freerw, int size) {
	FontLoadRWArgs args;
	args.rw = rw;
	args.freerw = freerw;
	args.size = size;
	args.result = NULL;
	args.done = CreateEvent(NULL, FALSE, FALSE, NULL);
	if (!args.done) return NULL;

	// Use STACK_SIZE_PARAM_IS_A_RESERVATION (0x00010000) to ensure the stack size is reserved.
	HANDLE thread = CreateThread(NULL, 64*1024*1024, loadFontRWThread, &args, 0x00010000, NULL);
	if (!thread) {
		CloseHandle(args.done);
		return NULL;
	}
	WaitForSingleObject(args.done, 30000);
	CloseHandle(args.done);
	CloseHandle(thread);
	return args.result;
}
*/
import "C"
import (
	"fmt"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"
)

func openFontSafe(path string, size int) (*ttf.Font, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	f := C.openFontOnBigStack(cpath, C.int(size))
	if f == nil {
		return nil, fmt.Errorf("TTF_OpenFont failed")
	}
	return (*ttf.Font)(unsafe.Pointer(f)), nil
}

func openFontFromMem(data []byte, size int) (*ttf.Font, error) {
	rw, err := sdl.RWFromMem(data)
	if err != nil {
		return nil, fmt.Errorf("RWFromMem failed: %w", err)
	}
	f := C.openFontRWOnBigStack((*C.SDL_RWops)(unsafe.Pointer(rw)), C.int(1), C.int(size))
	if f == nil {
		// If f is nil, the thread might have failed or TTF_OpenFontRW failed.
		// Since we passed freerw=1, TTF_OpenFontRW should have freed it,
		// but if the thread failed to start, we must free it here.
		rw.Close()
		return nil, fmt.Errorf("TTF_OpenFontRW failed")
	}
	return (*ttf.Font)(unsafe.Pointer(f)), nil
}
