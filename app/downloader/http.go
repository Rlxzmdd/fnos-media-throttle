package downloader

import (
	"errors"
	"net/url"
)

func httpEndpoint(base, suffix string) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("invalid service URL")
	}
	parsed.Path = suffix
	parsed.RawQuery = ""
	return parsed.String(), nil
}
