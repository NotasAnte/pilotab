package pilot

import "regexp"

type redaction struct {
	pattern     *regexp.Regexp
	replacement string
}

var redactions = []redaction{
	{regexp.MustCompile(`(?im)^(\s*(?:-\s*)?"?(?:authorization|proxy-authorization|cookie|set-cookie|x-api-key|x-auth-token|x-csrf-token)"?\s*[:=]\s*).+$`), "${1}[redacted]"},
	{regexp.MustCompile(`(?i)\b(bearer|basic)\s+[A-Za-z0-9\-._~+/]{8,}=*`), "$1 [redacted]"},
	{regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`), "[redacted-jwt]"},
	{regexp.MustCompile(`\b(?:ghp|gho|ghs|ghu|ghr|github_pat)_[A-Za-z0-9_]{20,}`), "[redacted-github-token]"},
	{regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`), "[redacted-aws-key]"},
	{regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`), "[redacted-google-key]"},
	{regexp.MustCompile(`\bxox[abposr]-[A-Za-z0-9-]{10,}`), "[redacted-slack-token]"},
	{regexp.MustCompile(`\bsk-(?:ant-|proj-)?[A-Za-z0-9_-]{20,}`), "[redacted-api-key]"},
}

func redact(text string) string {
	for _, r := range redactions {
		text = r.pattern.ReplaceAllString(text, r.replacement)
	}
	return text
}
