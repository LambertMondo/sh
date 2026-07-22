package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"
)

func main() {
	target, err := targetURL()
	if err != nil {
		log.Fatal(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:              "0.0.0.0:" + port,
		Handler:           newReverseProxy(target),
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	log.Printf("proxying http://%s to %s", server.Addr, target.Redacted())
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func targetURL() (*url.URL, error) {
	rawURL := strings.TrimSpace(os.Getenv("TARGET_URL"))
	if rawURL == "" {
		host := strings.TrimSpace(os.Getenv("TARGET_HOST"))
		if host == "" {
			return nil, errors.New("TARGET_URL or TARGET_HOST must be set")
		}

		port := strings.TrimSpace(os.Getenv("TARGET_PORT"))
		if port != "" && port != "443" {
			host = net.JoinHostPort(host, port)
		}
		rawURL = "https://" + host
	}

	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse TARGET_URL: %w", err)
	}
	if target.Scheme != "https" && target.Scheme != "http" {
		return nil, fmt.Errorf("TARGET_URL scheme must be http or https, got %q", target.Scheme)
	}
	if target.Hostname() == "" {
		return nil, errors.New("TARGET_URL must include a hostname")
	}
	if target.User != nil || target.Fragment != "" {
		return nil, errors.New("TARGET_URL must not contain credentials or a fragment")
	}

	return target, nil
}

func newReverseProxy(target *url.URL) *httputil.ReverseProxy {
	dialer := &net.Dialer{
		Timeout:   15 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			publicHost := request.In.Host
			publicProto := externalProtocol(request.In)
			forwardedFor := request.In.Header.Get("X-Forwarded-For")

			request.SetURL(target)
			request.Out.Host = target.Host
			request.SetXForwarded()
			request.Out.Header.Set("X-Forwarded-Host", publicHost)
			request.Out.Header.Set("X-Forwarded-Proto", publicProto)

			// Upsun's router supplies the original client chain. Preserve it and
			// append the router address added by SetXForwarded.
			if forwardedFor != "" {
				if proxyHop := request.Out.Header.Get("X-Forwarded-For"); proxyHop != "" {
					forwardedFor += ", " + proxyHop
				}
				request.Out.Header.Set("X-Forwarded-For", forwardedFor)
			}
		},
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
			ExpectContinueTimeout: time.Second,
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
		FlushInterval:  -1,
		ModifyResponse: rewriteResponse(target),
		ErrorHandler: func(writer http.ResponseWriter, request *http.Request, err error) {
			log.Printf("proxy %s %s: %v", request.Method, request.URL.RequestURI(), err)
			http.Error(writer, "Bad Gateway", http.StatusBadGateway)
		},
	}

	return proxy
}

func externalProtocol(request *http.Request) string {
	if protocol := strings.TrimSpace(strings.Split(request.Header.Get("X-Forwarded-Proto"), ",")[0]); protocol == "http" || protocol == "https" {
		return protocol
	}
	if request.TLS != nil {
		return "https"
	}
	return "http"
}

func rewriteResponse(target *url.URL) func(*http.Response) error {
	return func(response *http.Response) error {
		publicHost := response.Request.Header.Get("X-Forwarded-Host")
		publicProto := response.Request.Header.Get("X-Forwarded-Proto")

		if location := response.Header.Get("Location"); location != "" && publicHost != "" {
			parsed, err := url.Parse(location)
			if err == nil && strings.EqualFold(parsed.Hostname(), target.Hostname()) {
				parsed.Scheme = publicProto
				parsed.Host = publicHost
				response.Header.Set("Location", parsed.String())
			}
		}

		// A Domain=camerind.com cookie would be rejected by browsers visiting
		// the platformsh.site domain. Make such cookies host-only instead.
		if cookies := response.Cookies(); len(cookies) > 0 {
			changed := false
			for _, cookie := range cookies {
				if strings.EqualFold(strings.TrimPrefix(cookie.Domain, "."), target.Hostname()) {
					cookie.Domain = ""
					changed = true
				}
			}
			if changed {
				response.Header.Del("Set-Cookie")
				for _, cookie := range cookies {
					response.Header.Add("Set-Cookie", cookie.String())
				}
			}
		}

		return nil
	}
}
