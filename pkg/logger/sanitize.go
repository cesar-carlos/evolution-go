package logger

import "regexp"

var (
	// scheme://user:password@host
	userinfoRe = regexp.MustCompile(`(?i)((?:postgres(?:ql)?|amqp[s]?|https?|rediss?|mongodb)://[^:/@\s]+):([^@\s]+)@`)
	queryKeyRe = regexp.MustCompile(`(?i)([?&](?:token|apikey|api[_-]?key|password|passwd|secret|access[_-]?token|auth)=)([^&\s"',]*)`)
	bearerRe   = regexp.MustCompile(`(?i)(\bbearer\s+)([A-Za-z0-9._\-+=/]+)`)
	apikeyKVRe = regexp.MustCompile(`(?i)(\bapikey\s*[:=]\s*)([^\s"',]+)`)
)

// MaskSensitive redacts credentials commonly leaked in log lines (URL userinfo,
// query tokens, Bearer headers, apikey key/value). Safe to call on any string.
func MaskSensitive(s string) string {
	if s == "" {
		return s
	}
	s = userinfoRe.ReplaceAllString(s, `${1}:***@`)
	s = queryKeyRe.ReplaceAllString(s, `${1}***`)
	s = bearerRe.ReplaceAllString(s, `${1}***`)
	s = apikeyKVRe.ReplaceAllString(s, `${1}***`)
	return s
}
