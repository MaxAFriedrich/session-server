package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
)

func getValidBackendPaths(layout []Layout) []string {
	validPaths := make([]string, 0)
	for _, layout := range layout {
		for _, service := range layout.Services {
			validPaths = append(validPaths, fmt.Sprintf("/%s/%s", layout.ID, service.ID))
		}
	}
	return validPaths
}

func startsWithValidPath(path string, validPaths []string) bool {
	for _, validPath := range validPaths {
		if strings.HasPrefix(path, validPath) {
			return true
		}
	}
	return false
}

func registerProxy(backendTarget string, serverHost string, jwtSecret string, backendMap BackendMap, userInstances *map[UID]Instance, allInstances *map[Instance]bool) {
	backendSplit := strings.Split(backendTarget, "://")
	backendProtocol := backendSplit[0]
	backendHost := backendSplit[1]

	validBackendPaths := getValidBackendPaths(backendMap.Layout)

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			initialPath := req.URL.Path
			initialQuery := req.URL.RawQuery
			req.URL.Scheme = "http"
			req.URL.Host = serverHost
			req.URL.Path = "error"
			req.URL.RawQuery = "code=403&message=Failed to authenticate"

			jwtCookie, err := req.Cookie("SessionLogin")
			if err != nil {
				return
			}
			valid, user, err := verifyJwt(jwtCookie.Value, jwtSecret)

			if err != nil || !valid {
				return
			}
			// Check if the user has been allocated an instance, if not, return an error
			uid := buildUid(user)
			instance, err := uid.getInstance(userInstances, allInstances, backendMap.Backends)
			if err != nil {
				req.URL.RawQuery = "code=500&message=No instances available"
				log.Println("Used up all", len(*allInstances), "instances")
				return
			}

			unRoutedPath := strings.TrimPrefix(initialPath, "/proxy/")

			targetPath := fmt.Sprintf("/%s/%s", instance, unRoutedPath)
			// check for proxy path cookie

			proxyPathCookie, err := req.Cookie("Proxy-Path")

			notValidPath := !startsWithValidPath("/"+unRoutedPath, validBackendPaths)
			if err == nil {
				proxyPathInstance := strings.Split(proxyPathCookie.Value, "/")[1]
				isCorrectInstance := Instance(proxyPathInstance) == instance

				if notValidPath && isCorrectInstance {
					targetPath = "/" + unRoutedPath
				} else {
					proxyPathCookie.Value = ""
				}
			} else if notValidPath {
				req.URL.RawQuery = "code=403&message=Invalid path"
				return
			}

			req.URL.Scheme = backendProtocol
			req.URL.Host = backendHost
			req.URL.Path = targetPath
			req.URL.RawQuery = initialQuery

			log.Println("User", uid, "requested", initialPath, "routed to", targetPath, "on instance", instance, "with query", initialQuery, "request type", req.Method)
		},
		ModifyResponse: func(response *http.Response) error {
			return nil
		},
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true, // Disable keep-alives to ensure new connections for each request
		},
	}

	http.Handle("/proxy/", proxy)
}
