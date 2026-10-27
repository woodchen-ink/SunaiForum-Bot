package binance

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"SunaiForum-Bot/core"

	"github.com/adshao/go-binance/v2"
)

// getClient shares one client and connection pool across all Binance requests.
var getClient = sync.OnceValues(func() (*binance.Client, error) {
	return newBinanceClient(core.BinanceProxyURL)
})

// newBinanceClient configures only Binance traffic, never the global HTTP transport.
func newBinanceClient(rawProxyURL string) (*binance.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	rawProxyURL = strings.TrimSpace(rawProxyURL)
	if rawProxyURL != "" {
		proxyURL, err := url.Parse(rawProxyURL)
		if err != nil {
			// URL parsing errors can contain credentials; do not include the raw value.
			return nil, fmt.Errorf("invalid BINANCE_PROXY_URL: malformed proxy URL")
		}
		if proxyURL.Scheme != "socks5" && proxyURL.Scheme != "socks5h" {
			return nil, fmt.Errorf("invalid BINANCE_PROXY_URL: scheme must be socks5 or socks5h")
		}
		port, err := strconv.Atoi(proxyURL.Port())
		if proxyURL.Hostname() == "" || err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("invalid BINANCE_PROXY_URL: host and port (1-65535) are required")
		}
		if proxyURL.Path != "" || proxyURL.RawQuery != "" || proxyURL.ForceQuery || proxyURL.Fragment != "" {
			return nil, fmt.Errorf("invalid BINANCE_PROXY_URL: path, query and fragment are not allowed")
		}
		if proxyURL.User != nil {
			password, _ := proxyURL.User.Password()
			if len(proxyURL.User.Username()) > 255 || len(password) > 255 {
				return nil, fmt.Errorf("invalid BINANCE_PROXY_URL: credentials exceed SOCKS5 length limit")
			}
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	client := binance.NewClient("", "")
	client.HTTPClient = &http.Client{Transport: transport, Timeout: 15 * time.Second}
	return client, nil
}
