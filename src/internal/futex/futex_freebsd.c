//go:build none

#include <limits.h>
#include <stddef.h>
#include <stdint.h>
#include <time.h>
#include <sys/types.h>
#include <sys/umtx.h>

void tinygo_futex_wait(uint32_t *addr, uint32_t cmp) {
    struct _umtx_time timeout = {0};
    _umtx_op(addr, UMTX_OP_WAIT_UINT_PRIVATE, cmp, NULL, NULL);
}

void tinygo_futex_wait_timeout(uint32_t *addr, uint32_t cmp, uint64_t timeout_ns) {
    struct _umtx_time t = {0};
    t._timeout.tv_sec  = timeout_ns / 1000000000;
    t._timeout.tv_nsec = timeout_ns % 1000000000;
    t._flags = 0;
    t._clockid = CLOCK_MONOTONIC;
    _umtx_op(addr, UMTX_OP_WAIT_UINT_PRIVATE, cmp,
             (void *)sizeof(t), &t);
}

void tinygo_futex_wake(uint32_t *addr) {
    _umtx_op(addr, UMTX_OP_WAKE_PRIVATE, 1, NULL, NULL);
}

void tinygo_futex_wake_all(uint32_t *addr) {
    _umtx_op(addr, UMTX_OP_WAKE_PRIVATE, INT_MAX, NULL, NULL);
}
