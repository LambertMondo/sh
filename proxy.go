package main

import (
	"io"
	"log"
	"net"
	"os"
)

func main() {
	target := os.Getenv("TARGET_HOST") + ":" + os.Getenv("TARGET_PORT")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ln, err := net.Listen("tcp", "0.0.0.0:"+port)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go handle(conn, target)
	}
}

func handle(src net.Conn, target string) {
	defer src.Close()
	dst, err := net.Dial("tcp", target)
	if err != nil {
		log.Println(err)
		return
	}
	defer dst.Close()

	go io.Copy(dst, src)
	io.Copy(src, dst)
}
