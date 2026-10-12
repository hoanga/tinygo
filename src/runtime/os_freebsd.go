//go:build freebsd && !baremetal

package runtime

import "unsafe"

const GOOS = "freebsd"

const (
	// https://github.com/freebsd/freebsd-src/blob/main/sys/sys/mman.h
	flag_PROT_READ     = 0x1
	flag_PROT_WRITE    = 0x2
	flag_MAP_PRIVATE   = 0x2
	flag_MAP_ANONYMOUS = 0x1000 // MAP_ANON on FreeBSD
)

// https://github.com/freebsd/freebsd-src/blob/main/sys/sys/_timespec.h
const (
	clock_REALTIME      = 0
	clock_MONOTONIC_RAW = 4 // CLOCK_MONOTONIC_RAW (verify: FreeBSD uses CLOCK_MONOTONIC = 4)
)

const (
	sig_SIGBUS  = 10
	sig_SIGFPE  = 8
	sig_SIGILL  = 4
	sig_SIGSEGV = 11
)

// TODO: DOUBLE CHECK THIS IS THE SAME ON FREEBSD
// For the definition of the various header structs, see:
// https://refspecs.linuxfoundation.org/elf/elf.pdf
// Also useful:
// https://en.wikipedia.org/wiki/Executable_and_Linkable_Format
type elfHeader struct {
	ident_magic      uint32
	ident_class      uint8
	ident_data       uint8
	ident_version    uint8
	ident_osabi      uint8
	ident_abiversion uint8
	_                [7]byte // reserved
	filetype         uint16
	machine          uint16
	version          uint32
	entry            uintptr
	phoff            uintptr
	shoff            uintptr
	flags            uint32
	ehsize           uint16
	phentsize        uint16
	phnum            uint16
	shentsize        uint16
	shnum            uint16
	shstrndx         uint16
}

type elfProgramHeader64 struct {
	_type  uint32
	flags  uint32
	offset uintptr
	vaddr  uintptr
	paddr  uintptr
	filesz uintptr
	memsz  uintptr
	align  uintptr
}

type elfProgramHeader32 struct {
	_type  uint32
	offset uintptr
	vaddr  uintptr
	paddr  uintptr
	filesz uintptr
	memsz  uintptr
	flags  uint32
	align  uintptr
}

// ELF header of the currently running process.
//
//go:extern __ehdr_start
var ehdr_start elfHeader

// __errno_location equivalent on FreeBSD is __error()
//
//export __error
func libc_errno_location() *int32

// findGlobals finds globals in the .data/.bss sections.
// It parses the ELF program header to find writable segments.
func findGlobals(found func(start, end uintptr)) {
	// Relevant constants from the ELF specification.
	// See: https://refspecs.linuxfoundation.org/elf/elf.pdf
	const (
		PT_LOAD = 1
		PF_W    = 0x2 // program flag: write access
	)

	headerPtr := unsafe.Pointer(uintptr(unsafe.Pointer(&ehdr_start)) + ehdr_start.phoff)
	for i := 0; i < int(ehdr_start.phnum); i++ {
		// Look for a writable segment and scan its contents.
		// There is a little bit of duplication here, which is unfortunate. But
		// the alternative would be to put elfProgramHeader in separate files
		// which is IMHO a lot uglier. If only the ELF spec was consistent
		// between 32-bit and 64-bit...
		if TargetBits == 64 {
			header := (*elfProgramHeader64)(headerPtr)
			if header._type == PT_LOAD && header.flags&PF_W != 0 {
				start := header.vaddr
				end := start + header.memsz
				found(start, end)
			}
		} else {
			header := (*elfProgramHeader32)(headerPtr)
			if header._type == PT_LOAD && header.flags&PF_W != 0 {
				start := header.vaddr
				end := start + header.memsz
				found(start, end)
			}
		}
		headerPtr = unsafe.Add(headerPtr, ehdr_start.phentsize)
	}
}

// FreeBSD uses arc4random (provided by libc), not getrandom.
// No getrandom needed here; handled via crypto/rand.
//
//export getpagesize
func libc_getpagesize() int

//go:linkname syscall_Getpagesize syscall.Getpagesize
func syscall_Getpagesize() int {
	return libc_getpagesize()
}

func hardwareRand() (n uint64, ok bool) {
	// FreeBSD 12+ provides arc4random_buf in libc which is CSPRNG-backed.
	// Use it here rather than getrandom (which is Linux-specific).
	libc_arc4random_buf(unsafe.Pointer(&n), 8)
	return n, true
}

//export arc4random_buf
func libc_arc4random_buf(buf unsafe.Pointer, buflen uint)

//export fcntl
func libc_fcntl(fd int, cmd int, arg int) (ret int)

func fcntl(fd int32, cmd int32, arg int32) (ret int32, errno int32) {
	ret = int32(libc_fcntl(int(fd), int(cmd), int(arg)))
	errno = *libc_errno_location()
	return
}
