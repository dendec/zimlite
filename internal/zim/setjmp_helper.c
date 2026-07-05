#ifdef _WIN32
/*
 * FreeType (bundled statically inside libSDL2_ttf.a) references the symbol
 * `_setjmp`, which the GCC/Clang setjmp.h normally expands inline. The static
 * objects were built against a MinGW toolchain that emits an *external* call to
 * `_setjmp`, but zig's mingw runtime only exports the real implementation under
 * the names `__intrinsic_setjmp` / `__intrinsic_setjmpex` (see
 * lib/libc/mingw/misc/setjmp.S). The symbol `_setjmp` is therefore undefined at
 * link time.
 *
 * It is NOT correct to implement `_setjmp` as an ordinary C function:
 *
 *     int _setjmp(jmp_buf env, void *ctx) { return setjmp(env); }   // BROKEN
 *
 * `_setjmp` must capture the *caller's* register/stack state. A normal C
 * function captures its own (already-returned) frame, so the later `longjmp`
 * jumps into a destroyed stack frame and crashes with STATUS_ACCESS_VIOLATION
 * (0xC0000005). FreeType's cmap validation in tt_face_build_cmaps performs a
 * longjmp as normal control flow on essentially every font load, so this
 * corrupts the very first TTF_OpenFont call.
 *
 * The correct fix is to alias `_setjmp` to the real `__intrinsic_setjmp` via a
 * tail jump. Because a `jmp` (not `call`) does not push a return address, the
 * stack/return address seen by `__intrinsic_setjmp` is still the original
 * caller's, so it saves the correct context. `__intrinsic_setjmp` zeroes the
 * Frame field, which makes the matching mingw `longjmp` perform a plain
 * register/stack restore without SEH unwinding -- exactly what FreeType needs.
 */
#if defined(__x86_64__)
__asm__(
    ".globl _setjmp\n"
    "_setjmp:\n"
    "    jmp __intrinsic_setjmp\n"
);
#elif defined(__i386__)
__asm__(
    ".globl __setjmp\n"     /* leading underscore added by i386 PE mangling */
    "__setjmp:\n"
    "    jmp ___intrinsic_setjmp\n"
);
#endif
#endif
