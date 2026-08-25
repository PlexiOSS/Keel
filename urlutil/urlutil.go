package urlutil

import "net/url"

func DifferentHost(candidate, want string) bool {
	if candidate == "" {
		return true
	}

	wantURL, err := url.Parse(want)
	if err != nil || wantURL.Host == "" {
		return true
	}

	candidateURL, err := url.Parse(candidate)
	if err != nil {
		return true
	}

	return candidateURL.Host != wantURL.Host
}
