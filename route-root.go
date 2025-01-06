package main

import (
	"crypto/tls"
	"net/http"
	"net/http/httputil"
)

func registerRoot(serverHost string) {
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			if req.URL.Path == "/" {
				req.URL.Path = "/app/"
				req.URL.Host = serverHost
				req.URL.Scheme = "http"

				return
			} else if req.URL.Path == "/favicon.ico" || req.URL.Path == "/robots.txt" {
				req.URL.Host = serverHost
				req.URL.Scheme = "http"
				req.URL.Path = "/static" + req.URL.Path
				return
			}
			//	TODO
			//	verify user cookie
			// check if user has an instance set in ProxyPath cookie, if yes, and it matches, proxy to serverHost/proxy/path
		},
		ModifyResponse: func(response *http.Response) error {
			return nil
		},
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
		},
	}
	http.Handle("/", proxy)
}
