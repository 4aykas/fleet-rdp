package maker

import (
	"regexp"
	"strings"
)

// reserved: Windows device names. A file called CON, NUL, COM1... (with or
// without an extension) cannot be created.
var (
	reserved  = regexp.MustCompile(`(?i)^(con|prn|aux|nul|com[1-9]|lpt[1-9])(\..*)?$`)
	forbidden = regexp.MustCompile(`[\\/:*?"<>|\x00-\x1f]`)
	trailing  = regexp.MustCompile(`[. ]+$`)
)

// SafeName is the dashboard's safeName (dashboard/src/utils/download.ts): a
// name that is safe in a ZIP entry and a Windows file name. Forbidden
// characters become "_", trailing dots and spaces go, reserved device names
// get a "_" prefix, and an empty result is "launcher".
func SafeName(s string) string {
	c := trailing.ReplaceAllString(forbidden.ReplaceAllString(s, "_"), "")
	if c == "" {
		return "launcher"
	}
	if reserved.MatchString(c) {
		return "_" + c
	}
	return c
}

// bare drops a DOMAIN\ prefix.
func bare(login string) string { return login[strings.LastIndex(login, `\`)+1:] }

// FileBase is the launcher's file name without extension, as the dashboard's
// launcherFileBase: the login without its domain, then the label.
func FileBase(login, label string) string {
	return SafeName(bare(login)) + "_" + SafeName(label)
}

// SameLogin reports whether two logins name the same account file: the
// domain is dropped and case is ignored, as Windows does.
func SameLogin(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(bare(a)), strings.TrimSpace(bare(b)))
}
