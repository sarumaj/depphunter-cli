package cpp

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// cHeaders are the headers of the C standard library (C89 to C23).
var cHeaders = lang.WordSet(`assert.h complex.h ctype.h errno.h fenv.h float.h inttypes.h
iso646.h limits.h locale.h math.h setjmp.h signal.h stdalign.h stdarg.h stdatomic.h
stdbit.h stdbool.h stdckdint.h stddef.h stdint.h stdio.h stdlib.h stdnoreturn.h
string.h tgmath.h threads.h time.h uchar.h wchar.h wctype.h`)

// cppHeaders are the headers of the C++ standard library (C++98 to C++26), including
// the <cxxx> forms of the C headers.
var cppHeaders = lang.WordSet(`algorithm any array atomic barrier bit bitset charconv chrono
codecvt compare complex concepts condition_variable contracts coroutine debugging
deque exception execution expected filesystem flat_map flat_set format forward_list
fstream functional future generator hazard_pointer hive initializer_list inplace_vector
iomanip ios iosfwd iostream istream iterator latch limits linalg list locale map
mdspan memory memory_resource meta mutex new numbers numeric optional ostream print
queue random ranges ratio rcu regex scoped_allocator semaphore set shared_mutex simd
source_location span spanstream sstream stack stacktrace stdexcept stdfloat stop_token
streambuf string string_view strstream syncstream system_error text_encoding thread
tuple type_traits typeindex typeinfo unordered_map unordered_set utility valarray
variant vector version
cassert ccomplex cctype cerrno cfenv cfloat cinttypes ciso646 climits clocale cmath
csetjmp csignal cstdalign cstdarg cstdbool cstddef cstdint cstdio cstdlib cstring
ctgmath ctime cuchar cwchar cwctype`)

// systemHeaders are what the operating system or the compiler provides beside the
// standard library: POSIX, Linux, BSD and macOS, Windows, and compiler intrinsics.
// Windows headers are often written capitalized (<Windows.h>); the lookup ignores case.
var systemHeaders = lang.WordSet(strings.ToLower(`aio.h alloca.h cpio.h dirent.h dlfcn.h endian.h err.h
execinfo.h fcntl.h features.h fmtmsg.h fnmatch.h ftw.h getopt.h glob.h grp.h iconv.h
ifaddrs.h langinfo.h libgen.h libintl.h link.h malloc.h memory.h mntent.h monetary.h
mqueue.h ndbm.h netdb.h nl_types.h paths.h poll.h pthread.h pty.h pwd.h regex.h
resolv.h sched.h search.h semaphore.h shadow.h spawn.h strings.h stropts.h sysexits.h
syslog.h tar.h termio.h termios.h ucontext.h ulimit.h unistd.h utime.h utmp.h utmpx.h
wordexp.h byteswap.h error.h
windows.h winsock.h winsock2.h ws2tcpip.h windef.h winbase.h winerror.h winnt.h
winuser.h winreg.h wincrypt.h winioctl.h wininet.h winhttp.h tchar.h io.h direct.h
process.h conio.h crtdbg.h share.h objbase.h combaseapi.h shlobj.h shellapi.h
shlwapi.h commctrl.h commdlg.h iphlpapi.h psapi.h tlhelp32.h dbghelp.h mmsystem.h
ole2.h oleauto.h wtypes.h sddl.h aclapi.h lm.h ntstatus.h winternl.h versionhelpers.h
mswsock.h mstcpip.h afunix.h bcrypt.h ncrypt.h userenv.h wtsapi32.h
intrin.h immintrin.h emmintrin.h xmmintrin.h pmmintrin.h tmmintrin.h smmintrin.h
nmmintrin.h wmmintrin.h x86intrin.h cpuid.h arm_neon.h arm_acle.h
TargetConditionals.h AvailabilityMacros.h Availability.h
winapifamily.h fileapi.h wincon.h consoleapi.h processenv.h processthreadsapi.h
synchapi.h sysinfoapi.h handleapi.h errhandlingapi.h libloaderapi.h memoryapi.h
winsvc.h accctrl.h iptypes.h winperf.h
lwp.h thread.h pthread_np.h xlocale.h cxxabi.h unwind.h crt_externs.h kvm.h kstat.h
libutil.h util.h syscall.h xti.h port.h procinfo.h libperfstat.h`))

// systemDirectories are directories of system headers (<sys/socket.h>, <linux/fs.h>) and
// frameworks of the Apple SDKs (<CoreFoundation/CoreFoundation.h>).
var systemDirectories = lang.WordSet(`sys arpa net netinet netinet6 netpacket linux asm asm-generic
mach mach-o libkern machine xlocale os dispatch android gnu hurd uvm vm sanitizer
CoreFoundation CoreServices Foundation IOKit Security SystemConfiguration
ApplicationServices Cocoa AppKit Carbon`)

// standard returns the island a header the project does not have belongs to:
// ecosystemCStd, ecosystemCppStd, ecosystemSystem, or "" for a third-party library.
//
// Implements: REQ-CPP-005
func standard(name string) string {
	switch {
	case cHeaders[name]:
		return ecosystemCStd
	case cppHeaders[name], strings.HasPrefix(name, "experimental/"), strings.HasPrefix(name, "tr1/"),
		strings.HasPrefix(name, "ext/"),
		name == "bits/stdc++.h":
		return ecosystemCppStd
	case systemHeaders[strings.ToLower(name)], strings.HasPrefix(name, "bits/"): // <Windows.h> too
		return ecosystemSystem
	}
	if directory, _, ok := strings.Cut(name, "/"); ok && systemDirectories[directory] {
		return ecosystemSystem
	}
	return ""
}
