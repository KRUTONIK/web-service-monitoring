package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/KRUTONIK/web-service-monitoring/contracts/notifications"
)

type receivedMail struct {
	mailFrom string
	rcptTo   string
	data     string
}

func startSMTPServer(t *testing.T, rejectRecipient bool) (string, <-chan receivedMail) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	received := make(chan receivedMail, 1)
	go func() {
		defer close(received)
		defer listener.Close()
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "220 test SMTP\r\n")
		var mail receivedMail
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "EHLO "):
				_, _ = fmt.Fprint(connection, "250-test SMTP\r\n250 OK\r\n")
			case strings.HasPrefix(line, "MAIL FROM:"):
				mail.mailFrom = line
				_, _ = fmt.Fprint(connection, "250 sender OK\r\n")
			case strings.HasPrefix(line, "RCPT TO:"):
				mail.rcptTo = line
				if rejectRecipient {
					_, _ = fmt.Fprint(connection, "550 recipient rejected\r\n")
				} else {
					_, _ = fmt.Fprint(connection, "250 recipient OK\r\n")
				}
			case line == "DATA":
				_, _ = fmt.Fprint(connection, "354 send data\r\n")
				data, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				for data != ".\r\n" {
					mail.data += data
					data, err = reader.ReadString('\n')
					if err != nil {
						return
					}
				}
				_, _ = fmt.Fprint(connection, "250 queued\r\n")
				received <- mail
			case line == "QUIT":
				_, _ = fmt.Fprint(connection, "221 bye\r\n")
				return
			}
		}
	}()
	return listener.Addr().String(), received
}

func TestSendDeliversUTF8Message(t *testing.T) {
	address, received := startSMTPServer(t, false)
	event := notifications.Event{Type: "incident.started", ServiceID: "service-1", ServiceName: "Сервис", ServiceURL: "https://service.test", Recipient: "owner@example.test", OccurredAt: time.Date(2026, time.November, 5, 12, 0, 0, 0, time.UTC), Error: "Ошибка"}
	if err := Send(SMTPConfig{Address: address, From: "monitoring@example.test"}, event); err != nil {
		t.Fatalf("send: %v", err)
	}
	select {
	case mail := <-received:
		if mail.mailFrom != "MAIL FROM:<monitoring@example.test>" || mail.rcptTo != "RCPT TO:<owner@example.test>" {
			t.Fatalf("envelope = %+v", mail)
		}
		if !strings.Contains(mail.data, "Subject: =?UTF-8?B?") || !strings.Contains(mail.data, "Ошибка") || !strings.Contains(mail.data, "Сервис") {
			t.Fatalf("unexpected message: %q", mail.data)
		}
	case <-time.After(time.Second):
		t.Fatal("SMTP server did not receive the message")
	}
}

func TestSendReturnsSMTPRecipientRejection(t *testing.T) {
	address, _ := startSMTPServer(t, true)
	err := Send(SMTPConfig{Address: address, From: "monitoring@example.test"}, notifications.Event{Recipient: "owner@example.test"})
	if err == nil || !strings.Contains(err.Error(), "recipient rejected") {
		t.Fatalf("send = %v, want recipient rejection", err)
	}
}

func TestSendRejectsUnsupportedSTARTTLS(t *testing.T) {
	address, _ := startSMTPServer(t, false)
	err := Send(SMTPConfig{Address: address, From: "monitoring@example.test", TLS: true}, notifications.Event{Recipient: "owner@example.test"})
	if err == nil || !strings.Contains(err.Error(), "does not support STARTTLS") {
		t.Fatalf("send = %v", err)
	}
}
