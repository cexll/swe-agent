package github

import (
	"regexp"
	"strings"
)

var (
	reInvisible            = regexp.MustCompile("[\u200B\u200C\u200D\uFEFF]")
	reControl              = regexp.MustCompile("[\u0000-\u0008\u000B\u000C\u000E-\u001F\u007F-\u009F]")
	reSoftHyphen           = regexp.MustCompile("\u00AD")
	reBidi                 = regexp.MustCompile("[\u202A-\u202E\u2066-\u2069]")
	reMdImageAlt           = regexp.MustCompile(`!\[[^\]]*\]\(`)
	reMdLinkTitleDbl       = regexp.MustCompile(`(\[[^\]]*\]\([^)]+)\s+"[^"]*"`)
	reMdLinkTitleSgl       = regexp.MustCompile(`(\[[^\]]*\]\([^)]+)\s+'[^']*'`)
	reHTMLAttrAltDQ        = regexp.MustCompile(`\salt\s*=\s*"[^"]*"`)
	reHTMLAttrAltSQ        = regexp.MustCompile(`\salt\s*=\s*'[^']*'`)
	reHTMLAttrAltBare      = regexp.MustCompile(`\salt\s*=\s*[^\s>]+`)
	reHTMLAttrTitleDQ      = regexp.MustCompile(`\stitle\s*=\s*"[^"]*"`)
	reHTMLAttrTitleSQ      = regexp.MustCompile(`\stitle\s*=\s*'[^']*'`)
	reHTMLAttrTitleBare    = regexp.MustCompile(`\stitle\s*=\s*[^\s>]+`)
	reHTMLAttrAriaDQ       = regexp.MustCompile(`\saria-label\s*=\s*"[^"]*"`)
	reHTMLAttrAriaSQ       = regexp.MustCompile(`\saria-label\s*=\s*'[^']*'`)
	reHTMLAttrAriaBare     = regexp.MustCompile(`\saria-label\s*=\s*[^\s>]+`)
	reHTMLAttrDataDQ       = regexp.MustCompile(`\sdata-[a-zA-Z0-9-]+\s*=\s*"[^"]*"`)
	reHTMLAttrDataSQ       = regexp.MustCompile(`\sdata-[a-zA-Z0-9-]+\s*=\s*'[^']*'`)
	reHTMLAttrDataBare     = regexp.MustCompile(`\sdata-[a-zA-Z0-9-]+\s*=\s*[^\s>]+`)
	reHTMLAttrPlaceholderD = regexp.MustCompile(`\splaceholder\s*=\s*"[^"]*"`)
	reHTMLAttrPlaceholderS = regexp.MustCompile(`\splaceholder\s*=\s*'[^']*'`)
	reHTMLAttrPlaceholderB = regexp.MustCompile(`\splaceholder\s*=\s*[^\s>]+`)
	reHTMLComments         = regexp.MustCompile(`<!--[\s\S]*?-->`)

	reGitHubPATClassic   = regexp.MustCompile(`\bghp_[A-Za-z0-9]{36}\b`)
	reGitHubOAuth        = regexp.MustCompile(`\bgho_[A-Za-z0-9]{36}\b`)
	reGitHubInstallation = regexp.MustCompile(`\bghs_[A-Za-z0-9]{36}\b`)
	reGitHubRefresh      = regexp.MustCompile(`\bghr_[A-Za-z0-9]{36}\b`)
	reGitHubFineGrained  = regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{11,221}\b`)
)

// StripHtmlComments removes HTML comments.
func StripHtmlComments(s string) string {
	return reHTMLComments.ReplaceAllString(s, "")
}

// StripInvisibleCharacters removes zero-width and control chars.
func StripInvisibleCharacters(s string) string {
	s = reInvisible.ReplaceAllString(s, "")
	s = reControl.ReplaceAllString(s, "")
	s = reSoftHyphen.ReplaceAllString(s, "")
	s = reBidi.ReplaceAllString(s, "")
	return s
}

// StripMarkdownImageAltText removes alt text from markdown images.
func StripMarkdownImageAltText(s string) string { return reMdImageAlt.ReplaceAllString(s, "![](") }

// StripMarkdownLinkTitles removes title parts from markdown links.
func StripMarkdownLinkTitles(s string) string {
	s = reMdLinkTitleDbl.ReplaceAllString(s, "$1")
	s = reMdLinkTitleSgl.ReplaceAllString(s, "$1")
	return s
}

// StripHiddenAttributes removes potentially sensitive/hidden HTML attributes.
func StripHiddenAttributes(s string) string {
	s = reHTMLAttrAltDQ.ReplaceAllString(s, "")
	s = reHTMLAttrAltSQ.ReplaceAllString(s, "")
	s = reHTMLAttrAltBare.ReplaceAllString(s, "")
	s = reHTMLAttrTitleDQ.ReplaceAllString(s, "")
	s = reHTMLAttrTitleSQ.ReplaceAllString(s, "")
	s = reHTMLAttrTitleBare.ReplaceAllString(s, "")
	s = reHTMLAttrAriaDQ.ReplaceAllString(s, "")
	s = reHTMLAttrAriaSQ.ReplaceAllString(s, "")
	s = reHTMLAttrAriaBare.ReplaceAllString(s, "")
	s = reHTMLAttrDataDQ.ReplaceAllString(s, "")
	s = reHTMLAttrDataSQ.ReplaceAllString(s, "")
	s = reHTMLAttrDataBare.ReplaceAllString(s, "")
	s = reHTMLAttrPlaceholderD.ReplaceAllString(s, "")
	s = reHTMLAttrPlaceholderS.ReplaceAllString(s, "")
	s = reHTMLAttrPlaceholderB.ReplaceAllString(s, "")
	return s
}

// NormalizeHtmlEntities simplifies numeric entities for ASCII range.
func NormalizeHtmlEntities(s string) string {
	// decimal entities
	s = regexp.MustCompile(`&#(\d+);`).ReplaceAllStringFunc(s, func(in string) string {
		m := regexp.MustCompile(`\d+`).FindString(in)
		if m == "" {
			return ""
		}
		// safe parse
		var n int
		for i := 0; i < len(m); i++ {
			n = n*10 + int(m[i]-'0')
		}
		if n >= 32 && n <= 126 {
			return string(rune(n))
		}
		return ""
	})
	// hex entities
	s = regexp.MustCompile(`&#x([0-9a-fA-F]+);`).ReplaceAllStringFunc(s, func(in string) string {
		hex := regexp.MustCompile(`[0-9a-fA-F]+`).FindString(in)
		if hex == "" {
			return ""
		}
		var n int
		for i := 0; i < len(hex); i++ {
			c := hex[i]
			switch {
			case c >= '0' && c <= '9':
				n = n*16 + int(c-'0')
			case c >= 'a' && c <= 'f':
				n = n*16 + int(c-'a'+10)
			case c >= 'A' && c <= 'F':
				n = n*16 + int(c-'A'+10)
			}
		}
		if n >= 32 && n <= 126 {
			return string(rune(n))
		}
		return ""
	})
	return s
}

// RedactGitHubTokens censors GitHub token-like strings.
func RedactGitHubTokens(s string) string {
	s = reGitHubPATClassic.ReplaceAllString(s, "[REDACTED_GITHUB_TOKEN]")
	s = reGitHubOAuth.ReplaceAllString(s, "[REDACTED_GITHUB_TOKEN]")
	s = reGitHubInstallation.ReplaceAllString(s, "[REDACTED_GITHUB_TOKEN]")
	s = reGitHubRefresh.ReplaceAllString(s, "[REDACTED_GITHUB_TOKEN]")
	s = reGitHubFineGrained.ReplaceAllString(s, "[REDACTED_GITHUB_TOKEN]")
	return s
}

// SanitizeContent applies a conservative cleaning pipeline.
func SanitizeContent(s string) string {
	if s == "" {
		return s
	}
	s = StripHtmlComments(s)
	s = StripInvisibleCharacters(s)
	s = StripMarkdownImageAltText(s)
	s = StripMarkdownLinkTitles(s)
	s = StripHiddenAttributes(s)
	s = NormalizeHtmlEntities(s)
	s = RedactGitHubTokens(s)
	// Trim repeated whitespace edges but keep internal formatting
	return strings.TrimSpace(s)
}
