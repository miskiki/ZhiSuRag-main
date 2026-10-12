/*
 * emutls_stub.c — link fix for DuckDB prebuilt static libs on Windows/MinGW.
 *
 * duckdb-go-bindings ships libduckdb_*.a built with emulated TLS; objects such
 * as re2.cc.obj reference the emutls control symbols for libstdc++'s
 * call_once plumbing:
 *   __emutls_v._ZSt11__once_call    (std::__once_call, std::function<void()>)
 *   __emutls_v._ZSt15__once_callable(std::__once_callable, void*)
 * MSYS2 GCC 16's libstdc++ is not built with emutls, so nothing defines them.
 *
 * This TU defines the control blocks directly. Layout was verified against a
 * control block defined inside libduckdb_static.a (pg_functions.cpp.obj):
 *   struct { u64 size; u64 align; u64 index; void *templ; }  (32 bytes)
 * templ == NULL means zero-initialized storage at first use per thread,
 * matching libstdc++ semantics for these two variables.
 */

typedef struct {
    unsigned long long size;
    unsigned long long align;
    unsigned long long index;
    void *templ;
} emutls_control;

/* std::__once_call — std::function<void()>: 32 bytes, 8-aligned */
__attribute__((weak, used))
const emutls_control once_call_ctl
    __asm("__emutls_v._ZSt11__once_call") = {32, 8, 0, 0};

/* std::__once_callable — void*: 8 bytes, 8-aligned */
__attribute__((weak, used))
const emutls_control once_callable_ctl
    __asm("__emutls_v._ZSt15__once_callable") = {8, 8, 0, 0};
