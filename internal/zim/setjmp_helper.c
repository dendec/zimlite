#ifdef _WIN32
#include <setjmp.h>
#include <stddef.h>

/*
 * Some versions of SDL2_ttf or other libraries might be compiled against a MinGW 
 * version that expects _setjmp to be a function, but in Zig/Clang it might be 
 * an intrinsic or a macro. This wrapper provides the symbol.
 */
#undef _setjmp
int _setjmp(jmp_buf env, void *ctx) {
    return setjmp(env);
}
#endif
