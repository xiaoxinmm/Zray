// Package camo implements HTTP camouflage for ZRay connections.
package camo

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"strings"
)

var commonPaths = []string{
	"/", "/index.html", "/api/v1/status", "/search?q=keyword",
	"/blog/2024/01/welcome", "/static/css/main.css", "/ws",
	"/login", "/dashboard", "/assets/logo.png",
	"/cdn-cgi/trace", "/favicon.ico", "/robots.txt",
	"/api/v2/health", "/.well-known/acme-challenge",
}

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"

// WriteHTTPCamo writes a fake HTTP request header to disguise the connection.
func WriteHTTPCamo(w io.Writer, serverHost string) error {
	path := commonPaths[rand.Intn(len(commonPaths))]
	var b strings.Builder
	fmt.Fprintf(&b, "GET %s HTTP/1.1\r\n", path)
	fmt.Fprintf(&b, "Host: %s\r\n", serverHost)
	b.WriteString("Connection: keep-alive\r\n")
	b.WriteString("Cache-Control: max-age=0\r\n")
	b.WriteString("sec-ch-ua: \"Chromium\";v=\"122\", \"Not(A:Brand\";v=\"24\", \"Google Chrome\";v=\"122\"\r\n")
	b.WriteString("sec-ch-ua-mobile: ?0\r\n")
	b.WriteString("sec-ch-ua-platform: \"Windows\"\r\n")
	b.WriteString("Upgrade-Insecure-Requests: 1\r\n")
	fmt.Fprintf(&b, "User-Agent: %s\r\n", userAgent)
	b.WriteString("Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8\r\n")
	b.WriteString("Sec-Fetch-Site: none\r\n")
	b.WriteString("Sec-Fetch-Mode: navigate\r\n")
	b.WriteString("Sec-Fetch-User: ?1\r\n")
	b.WriteString("Sec-Fetch-Dest: document\r\n")
	b.WriteString("Accept-Encoding: gzip, deflate, br\r\n")
	b.WriteString("Accept-Language: en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7\r\n")
	b.WriteString("\r\n")
	_, err := w.Write([]byte(b.String()))
	return err
}

// StripHTTPCamo reads and discards a fake HTTP request header.
func StripHTTPCamo(br *bufio.Reader) error {
	peek, _ := br.Peek(4)
	method := string(peek)
	if method == "GET " || method == "POST" || method == "HEAD" || method == "PUT " || method == "OPTI" {
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				return err
			}
			if line == "\r\n" || line == "\n" {
				return nil
			}
		}
	}
	return nil // no camo header present
}
