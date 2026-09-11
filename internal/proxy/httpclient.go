package proxy

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

func NewHTTPClient(proxyURL string, timeout time.Duration) *http.Client {
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	if proxyURL == "" {
		return &http.Client{Timeout: timeout}
	}

	raw := proxyURL
	if !strings.Contains(raw, "://") {
		raw = "socks5://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		log.Printf("proxy: invalid PROXY_URL %q, using direct connection: %v", proxyURL, err)
		return &http.Client{Timeout: timeout}
	}

	switch u.Scheme {
	case "socks5", "socks5h":
		return socks5Client(u, timeout)
	case "http", "https":
		return httpProxyClient(u, timeout)
	default:
		log.Printf("proxy: unsupported scheme %q, using direct connection", u.Scheme)
		return &http.Client{Timeout: timeout}
	}
}

func socks5Client(u *url.URL, timeout time.Duration) *http.Client {
	host := u.Host
	if host == "" {
		log.Printf("proxy: empty proxy host, using direct connection")
		return &http.Client{Timeout: timeout}
	}

	var auth *proxy.Auth
	if u.User != nil {
		pw, _ := u.User.Password()
		auth = &proxy.Auth{User: u.User.Username(), Password: pw}
	}

	dialer, err := proxy.SOCKS5("tcp", host, auth, proxy.Direct)
	if err != nil {
		log.Printf("proxy: socks5 dialer error, using direct connection: %v", err)
		return &http.Client{Timeout: timeout}
	}

	log.Printf("proxy: using SOCKS5 %s", host)
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				_ = ctx
				return dialer.Dial(network, address)
			},
		},
	}
}

func httpProxyClient(u *url.URL, timeout time.Duration) *http.Client {
	log.Printf("proxy: using HTTP proxy %s", u.Host)
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(u),
		},
	}
}

// MaskProxyURL hides credentials in proxy URL for logging.
func MaskProxyURL(proxyURL string) string {
	if proxyURL == "" {
		return ""
	}
	raw := proxyURL
	if !strings.Contains(raw, "://") {
		raw = "socks5://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "(invalid)"
	}
	if u.User != nil {
		u.User = url.UserPassword("***", "***")
	}
	return u.String()
}

// ValidateProxy checks that proxy can reach Telegram API.
func ValidateProxy(client *http.Client) error {
	req, err := http.NewRequest(http.MethodGet, "https://api.telegram.org", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach Telegram API through proxy: %w", err)
	}
	resp.Body.Close()
	return nil
}
