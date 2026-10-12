//go:build freebsd && !baremetal

package task

import "unsafe"

// FreeBSD uses a pointer type for pthread_t:
//
//	typedef struct pthread *pthread_t;
type threadID unsafe.Pointer
