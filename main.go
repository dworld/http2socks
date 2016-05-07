package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"

	"h12.me/socks"
)

var _ = fmt.Printf
var _ = log.Printf

//HTTP2Socks is a http handler proxy http request to socks
type HTTP2Socks struct {
	SocksAddr  string // socks proxy address
	SocksProto int    // socks proxy protocol type
}

// Copy check return value of io.Copy
func Copy(dst io.Writer, src io.Reader) {
	_, err := io.Copy(dst, src)
	if err != nil {
		log.Printf("copy connection error, %s", err)
	}
}

func (s *HTTP2Socks) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	log.Printf("REQUEST: %s %s", r.Method, r.RequestURI)
	dialer := socks.DialSocksProxy(s.SocksProto, s.SocksAddr)
	if r.Method == "CONNECT" {
		hj, ok := rw.(http.Hijacker)
		if !ok {
			log.Printf("can't cast to Hijacker")
			return
		}
		conn, bufrw, err := hj.Hijack()
		if err != nil {
			log.Printf("can't hijack the connection")
			return
		}
		_, err = bufrw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		if err != nil {
			log.Printf("write CONNECT response header error, %s", err)
			return
		}
		err = bufrw.Flush()
		if err != nil {
			log.Printf("flush error, %s", err)
			return
		}
		outconn, err := dialer("tcp", r.Host)
		if err != nil {
			log.Printf("dial to %s error, %s", r.Host, err)
			return
		}
		go Copy(conn, outconn)
		go Copy(outconn, conn)
		return
	}
	tr := &http.Transport{
		Dial: dialer,
	}
	client := &http.Client{
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}
	outreq := *r
	outreq.RequestURI = ""
	resp, err := client.Do(&outreq)
	if err != nil {
		log.Printf("request socks: %s", err)
		return
	}
	defer resp.Body.Close()
	_, err = io.Copy(rw, resp.Body)
	if err != nil {
		log.Printf("write response: %s", err)
		return
	}
}

var (
	help      = flag.Bool("help", false, "show this help")
	addr      = flag.String("addr", "127.0.0.1:8081", "address listen to")
	sockAddr  = flag.String("socks_addr", "192.168.56.1:8080", "socks server addr")
	sockProto = flag.String("socks_proto", "socks5", "socks protocol type, [socks5|socks4|socks4a]")
)

func main() {
	flag.Parse()
	if *help {
		flag.PrintDefaults()
		return
	}
	var socksType int
	switch *sockProto {
	case "socks5":
		socksType = socks.SOCKS5
	case "socks4":
		socksType = socks.SOCKS4
	case "socks4a":
		socksType = socks.SOCKS4A
	default:
		fmt.Printf("invalid socks protocol")
		return
	}
	s := &HTTP2Socks{
		SocksAddr:  *sockAddr,
		SocksProto: socksType,
	}
	log.Printf("serve http proxy on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, s))
}
