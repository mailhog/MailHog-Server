package smtp

import (
	"crypto/tls"
	"io"
	"log"
	"net"

	"github.com/mailhog/MailHog-Server/config"
)

func Listen(cfg *config.Config, exitCh chan int) *net.TCPListener {
	log.Printf("[SMTP] Binding to address: %s\n", cfg.SMTPBindAddr)
	ln, err := net.Listen("tcp", cfg.SMTPBindAddr)
	if err != nil {
		log.Fatalf("[SMTP] Error listening on socket: %s\n", err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("[SMTP] Error accepting connection: %s\n", err)
			continue
		}

		if cfg.Monkey != nil {
			ok := cfg.Monkey.Accept(conn)
			if !ok {
				conn.Close()
				continue
			}
		}

		var tlsUpgrader func() io.ReadWriteCloser
		if cfg.TLSConfig != nil {
			tlsUpgrader = func() io.ReadWriteCloser {
				return io.ReadWriteCloser(tls.Server(conn, cfg.TLSConfig))
			}
		}

		go Accept(
			conn.(*net.TCPConn).RemoteAddr().String(),
			io.ReadWriteCloser(conn),
			tlsUpgrader,
			cfg.Storage,
			cfg.MessageChan,
			cfg.Hostname,
			cfg.Monkey,
		)
	}
}
