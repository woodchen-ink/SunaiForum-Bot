package binance

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestNewBinanceClient validates configuration without exposing proxy credentials.
func TestNewBinanceClient(t *testing.T) {
	for _, raw := range []string{
		"http://proxy:1080", "proxy:1080", "socks5://proxy", "socks5://:1080",
		"socks5://proxy:0", "socks5://proxy:65536", "socks5://proxy:abc",
		"socks5://proxy:1080/path", "socks5://proxy:1080?x=1", "socks5://proxy:1080#x",
		"socks5://user:secret%zz@proxy:1080", "socks5://" + strings.Repeat("a", 256) + ":secret@proxy:1080",
	} {
		client, err := newBinanceClient(raw)
		if err == nil || client != nil {
			t.Fatalf("invalid configuration accepted")
		}
		if strings.Contains(err.Error(), "secret") {
			t.Fatal("configuration error leaked password")
		}
	}

	for _, raw := range []string{"", "  ", "socks5://proxy:1080", "socks5h://user:pass@proxy:1080", "socks5://[::1]:1080"} {
		client, err := newBinanceClient(raw)
		if err != nil {
			t.Fatalf("valid configuration rejected: %v", err)
		}
		transport := client.HTTPClient.Transport.(*http.Transport)
		defer transport.CloseIdleConnections()
		if transport == http.DefaultTransport || client.HTTPClient == http.DefaultClient {
			t.Fatal("global HTTP defaults must not be modified")
		}
		if client.HTTPClient.Timeout != 15*time.Second {
			t.Fatal("missing request timeout")
		}
	}
}

// TestBinanceSOCKS5 exercises real SDK requests, remote DNS and password authentication.
func TestBinanceSOCKS5(t *testing.T) {
	for _, tc := range []struct {
		name, scheme, user, password string
		reject                       bool
	}{
		{name: "no_auth", scheme: "socks5"},
		{name: "password", scheme: "socks5", user: "alice", password: "p@ss:# word"},
		{name: "socks5h_password", scheme: "socks5h", user: "alice", password: "secret"},
		{name: "wrong_password", scheme: "socks5", user: "alice", password: "wrong", reject: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			result := make(chan error, 1)
			go func() { result <- serveTestSOCKS(listener, tc.user, tc.password, tc.reject) }()
			proxyURL := &url.URL{Scheme: tc.scheme, Host: listener.Addr().String()}
			if tc.user != "" {
				proxyURL.User = url.UserPassword(tc.user, tc.password)
			}
			client, err := newBinanceClient(proxyURL.String())
			if err != nil {
				t.Fatal(err)
			}
			defer client.HTTPClient.CloseIdleConnections()
			client.BaseURL = "http://binance.invalid"
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			prices, requestErr := client.NewListPricesService().Symbol("BTCUSDT").Do(ctx)
			if tc.reject {
				if requestErr == nil {
					t.Fatal("rejected authentication unexpectedly succeeded")
				}
			} else if requestErr != nil || len(prices) != 1 || prices[0].Price != "123.45" {
				t.Fatalf("proxy SDK request failed: %v", requestErr)
			}
			select {
			case err := <-result:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("SOCKS server did not complete")
			}
		})
	}
}

// serveTestSOCKS accepts one SOCKS5 connection and responds as a mock Binance endpoint.
func serveTestSOCKS(listener net.Listener, user, password string, reject bool) error {
	conn, err := listener.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return err
	}
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return err
	}
	if header[0] != 5 {
		return fmt.Errorf("wrong SOCKS version")
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return err
	}
	method := byte(0)
	if user != "" {
		method = 2
	}
	if _, err := conn.Write([]byte{5, method}); err != nil {
		return err
	}
	if method == 2 {
		if _, err := io.ReadFull(conn, header); err != nil {
			return err
		}
		if header[0] != 1 {
			return fmt.Errorf("wrong authentication version")
		}
		username := make([]byte, int(header[1]))
		if _, err := io.ReadFull(conn, username); err != nil {
			return err
		}
		length := make([]byte, 1)
		if _, err := io.ReadFull(conn, length); err != nil {
			return err
		}
		secret := make([]byte, int(length[0]))
		if _, err := io.ReadFull(conn, secret); err != nil {
			return err
		}
		if string(username) != user || string(secret) != password {
			return fmt.Errorf("credentials not transmitted intact")
		}
		status := byte(0)
		if reject {
			status = 1
		}
		if _, err := conn.Write([]byte{1, status}); err != nil {
			return err
		}
		if reject {
			return nil
		}
	}
	request := make([]byte, 5)
	if _, err := io.ReadFull(conn, request); err != nil {
		return err
	}
	if request[0] != 5 || request[1] != 1 || request[3] != 3 {
		return fmt.Errorf("expected SOCKS CONNECT with remote DNS")
	}
	destination := make([]byte, int(request[4])+2)
	if _, err := io.ReadFull(conn, destination); err != nil {
		return err
	}
	if string(destination[:len(destination)-2]) != "binance.invalid" {
		return fmt.Errorf("unexpected proxy destination")
	}
	if _, err := conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 80}); err != nil {
		return err
	}
	httpRequest, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		return err
	}
	defer httpRequest.Body.Close()
	if httpRequest.URL.Path != "/api/v3/ticker/price" || httpRequest.URL.Query().Get("symbol") != "BTCUSDT" {
		return fmt.Errorf("unexpected SDK request")
	}
	body := `{"symbol":"BTCUSDT","price":"123.45"}`
	_, err = fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nConnection: close\r\nContent-Length: %d\r\n\r\n%s", len(body), body)
	return err
}
