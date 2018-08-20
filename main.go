package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"

	log "github.com/golang/glog"
	"github.com/satori/go.uuid"
)

var (
	addr             = flag.String("addr", ":8080", "The binding for this proxy")
	dest             = flag.String("dest", "http://localhost:9000", "Proxied destination")
	basicAuthAllowed = strings.Split(os.Getenv("BASIC_AUTH_ALLOWED"), ",")
	openEndpoints    = strings.Split(os.Getenv("OPEN_ENDPOINTS"), ",")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "The following environment variables must be set:\n\n")
		fmt.Fprintln(os.Stderr, "  BASIC_AUTH_ALLOWED")
		fmt.Fprintln(os.Stderr, "    a list of comma-separated basic auth user:pass pairs")
		fmt.Fprintln(os.Stderr, "  OPEN_ENDPOINTS")
		fmt.Fprintln(os.Stderr, "    a list of comma-separated pathnames")
		fmt.Fprintf(os.Stderr, "\nOther flags:\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
}

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func delHopHeaders(header http.Header) {
	for _, h := range hopHeaders {
		header.Del(h)
	}
}

func appendHostToXForwardHeader(header http.Header, host string) {
	// If we aren't the first proxy retain prior
	// X-Forwarded-For information as a comma+space
	// separated list and fold multiple headers into one.
	if prior, ok := header["X-Forwarded-For"]; ok {
		host = strings.Join(prior, ", ") + ", " + host
	}
	header.Set("X-Forwarded-For", host)
}

func isPrivate(targetPath string) bool {
	for _, path := range openEndpoints {
		re, _ := regexp.Compile(path)
		if re.MatchString(targetPath) {
			return false
		}
	}
	return true
}

func isAuthenticated(r *http.Request) bool {
	username, password, authOK := r.BasicAuth()
	if authOK == false {
		return false
	}

	for _, basicAuthCreds := range basicAuthAllowed {
		creds := strings.Split(basicAuthCreds, ":")
		if username == creds[0] && password == creds[1] {
			return true
		}
	}
	return false
}

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type proxy struct {
	httpClient
}

func (p *proxy) ServeHTTP(wr http.ResponseWriter, inboundReq *http.Request) {

	uuid := uuid.NewV4()

	log.Infof("requestId=%v %v %v", uuid, inboundReq.Method, inboundReq.URL.String())

	if isAuthenticated(inboundReq) {
		// continue
	} else if isPrivate(inboundReq.URL.Path) {
		msg := "not authorized"
		http.Error(wr, msg, http.StatusUnauthorized)
		log.Infof("requestId=%v %v", uuid, msg)
		return
	}

	var outboundReqBody io.Reader

	// We want to fully read a non-nil body of an inbound request to avoid having
	// the `Transfer-Encoding: chunked` header set when `client.Do(req)` is called.
	// See documentation of `transferWriter.shouldSendChunkedRequestBody()`.
	// In particular, HTTP handlers backed by gRPC gateway will not pass bodies
	// through if this header is set.
	if inboundReq.Body != nil {
		inboundReqBody, err := ioutil.ReadAll(inboundReq.Body)
		if err != nil {
			log.Warningf(
				"requestId=%v Unable to read body from inboud request: %v",
				uuid,
				err)
		}
		outboundReqBody = bytes.NewReader(inboundReqBody)
	}

	outboundReq, _ := http.NewRequest(
		inboundReq.Method,
		*dest+inboundReq.URL.String(),
		outboundReqBody)

	copyHeader(outboundReq.Header, inboundReq.Header)

	if outboundReq.URL.Scheme != "http" && outboundReq.URL.Scheme != "https" {
		msg := "unsupported protocol scheme " + outboundReq.URL.Scheme
		http.Error(wr, msg, http.StatusBadRequest)
		log.Warningf("requestId=%v %v", uuid, msg)
		return
	}

	clientIP, _, _ := net.SplitHostPort(outboundReq.Host)
	appendHostToXForwardHeader(outboundReq.Header, clientIP)

	delHopHeaders(outboundReq.Header)

	resp, err := p.Do(outboundReq)
	if err != nil {
		http.Error(wr, "Server Error", http.StatusInternalServerError)
		log.Errorf("requestId=%v ServeHTTP: %v", uuid, err)
		return
	}

	defer resp.Body.Close()

	log.Infof("requestId=%v %v %v", uuid, outboundReq.URL, resp.Status)

	delHopHeaders(resp.Header)

	copyHeader(wr.Header(), resp.Header)

	wr.WriteHeader(resp.StatusCode)
	io.Copy(wr, resp.Body)
}

func main() {
	handler := &proxy{&http.Client{}}
	log.Infof("Listening on %v, proxying requests for %v", *addr, *dest)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
