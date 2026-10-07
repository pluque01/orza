//go:build darwin && cgo

package credential

/*
#cgo LDFLAGS: -framework CoreFoundation -framework Security
#include <CoreFoundation/CoreFoundation.h>
#include <Security/Security.h>
#include <stdlib.h>

static char *orza_ui_string(CFStringRef value) {
	CFIndex size = CFStringGetMaximumSizeForEncoding(CFStringGetLength(value), kCFStringEncodingUTF8) + 1;
	char *buffer = malloc(size);
	if (buffer == NULL) abort();
	if (!CFStringGetCString(value, buffer, size, kCFStringEncodingUTF8)) abort();
	return buffer;
}

static char *orza_ui_key(void) { return orza_ui_string(kSecUseAuthenticationUI); }
static char *orza_ui_fail(void) { return orza_ui_string(kSecUseAuthenticationUIFail); }
*/
import "C"

import "unsafe"

// The pinned wrapper converts SetString keys/values into CFStrings, which
// Security.framework compares by value. Obtain the actual native constants,
// not guessed spellings or kSecUseAuthenticationUISkip (which hides matches).
func darwinAuthenticationUI() (string, string) {
	key, value := C.orza_ui_key(), C.orza_ui_fail()
	defer C.free(unsafe.Pointer(key))
	defer C.free(unsafe.Pointer(value))
	return C.GoString(key), C.GoString(value)
}
