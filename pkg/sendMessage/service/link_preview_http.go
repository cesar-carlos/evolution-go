package send_service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// newLinkPreviewHTTPClient validates the actual dial destination, including DNS
// and redirects. Private resources require explicit operator opt-in.
func newLinkPreviewHTTPClient(allowPrivate bool) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, errors.New("preview destination unavailable")
		}
		if len(ips) == 0 {
			return nil, errors.New("preview destination unavailable")
		}
		for _, ip := range ips {
			if !allowPrivate && !publicPreviewIP(ip.IP) {
				return nil, errors.New("private preview destination disabled")
			}
		}
		for _, ip := range ips {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
		}
		return nil, errors.New("preview connection failed")
	}
	return &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("preview redirect limit reached")
		}
		return validatePreviewURL(req.URL)
	}}
}

func publicPreviewIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32"} {
		if netip.MustParsePrefix(prefix).Contains(addr) {
			return false
		}
	}
	return true
}

func validatePreviewURL(target *url.URL) error {
	if (target.Scheme != "http" && target.Scheme != "https") || target.Hostname() == "" || target.User != nil {
		return errors.New("preview requires HTTP(S) URL without credentials")
	}
	return nil
}

// fetchLinkPreviewResource retains the final URL for resolving relative metadata.
func fetchLinkPreviewResource(ctx context.Context, client *http.Client, target string, maxBytes int64) ([]byte, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, nil, errors.New("invalid preview URL")
	}
	if err := validatePreviewURL(req.URL); err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", linkPreviewUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, errors.New("preview request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, nil, fmt.Errorf("preview returned HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, nil, err
	}
	if int64(len(raw)) > maxBytes {
		return nil, nil, errors.New("preview resource exceeds byte limit")
	}
	return raw, resp.Request.URL, nil
}

func fetchLinkMetadata(ctx context.Context, client *http.Client, target string) (string, string, string, error) {
	raw, finalURL, err := fetchLinkPreviewResource(ctx, client, target, linkPreviewMaxHTMLBytes)
	if err != nil {
		return "", "", "", err
	}
	tokenizer := html.NewTokenizer(bytes.NewReader(raw))
	title, ogTitle, description, imageURL := "", "", "", ""
	inTitle := false
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			if err := tokenizer.Err(); err != io.EOF {
				return "", "", "", err
			}
			if ogTitle != "" {
				title = ogTitle
			}
			return title, description, absoluteURL(finalURL.String(), imageURL), nil
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			if token.Data == "title" {
				inTitle = true
			}
			if token.Data != "meta" {
				continue
			}
			property, content := "", ""
			for _, attr := range token.Attr {
				if attr.Key == "property" || attr.Key == "name" {
					property = strings.ToLower(attr.Val)
				}
				if attr.Key == "content" {
					content = attr.Val
				}
			}
			switch property {
			case "og:title":
				ogTitle = content
			case "og:description":
				description = content
			case "description":
				if description == "" {
					description = content
				}
			case "og:image":
				imageURL = content
			}
		case html.EndTagToken:
			if tokenizer.Token().Data == "title" {
				inTitle = false
			}
		case html.TextToken:
			if inTitle {
				title += string(tokenizer.Text())
			}
		}
	}
}
