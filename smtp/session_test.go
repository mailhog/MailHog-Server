package smtp

import (
	"errors"
	"io"
	"sync"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/mailhog/data"
	"github.com/mailhog/storage"
)

type fakeRw struct {
	_read  func(p []byte) (n int, err error)
	_write func(p []byte) (n int, err error)
	_close func() error
}

func (rw *fakeRw) Read(p []byte) (n int, err error) {
	if rw._read != nil {
		return rw._read(p)
	}
	return 0, nil
}
func (rw *fakeRw) Close() error {
	if rw._close != nil {
		return rw._close()
	}
	return nil
}
func (rw *fakeRw) Write(p []byte) (n int, err error) {
	if rw._write != nil {
		return rw._write(p)
	}
	return len(p), nil
}

func TestAccept(t *testing.T) {
	Convey("Accept should handle a connection", t, func() {
		frw := &fakeRw{}
		mChan := make(chan *data.Message)
		Accept("1.1.1.1:11111", frw, storage.CreateInMemory(), mChan, "localhost", nil)
	})
}

func TestSocketError(t *testing.T) {
	Convey("Socket errors should return from Accept", t, func() {
		frw := &fakeRw{
			_read: func(p []byte) (n int, err error) {
				return -1, errors.New("OINK")
			},
		}
		mChan := make(chan *data.Message)
		Accept("1.1.1.1:11111", frw, storage.CreateInMemory(), mChan, "localhost", nil)
	})
}

func TestAcceptMessage(t *testing.T) {
	Convey("acceptMessage should be called", t, func() {
		mbuf := "EHLO localhost\r\n" +
			"MAIL FROM:<test>\r\n" +
			"RCPT TO:<test>\r\n" +
			"DATA\r\n" +
			"Hi.\r\n" +
			".\r\n" +
			"QUIT\n"

		frw, obuf := getBuffer(mbuf)
		mChan := make(chan *data.Message)
		var wg sync.WaitGroup
		wg.Add(1)
		handlerCalled := false
		var storedMessage *data.Message
		go func() {
			handlerCalled = true
			storedMessage = <-mChan
			wg.Done()
		}()
		Accept("1.1.1.1:11111", frw, storage.CreateInMemory(), mChan, "localhost", nil)
		wg.Wait()

		So(handlerCalled, ShouldBeTrue)

		So(storedMessage, ShouldNotBeNil)
		So(string(*obuf), ShouldEqual,
			"220 localhost ESMTP MailHog\r\n"+
				"250-Hello localhost\r\n"+
				"250-PIPELINING\r\n"+
				"250 AUTH PLAIN\r\n"+
				"250 Sender test ok\r\n"+
				"250 Recipient test ok\r\n"+
				"354 End data with <CR><LF>.<CR><LF>\r\n"+
				"250 Ok: queued as "+storedMessage.ID+"\r\n",
		)
	})
}

func getBuffer(input string) (io.ReadWriteCloser, *[]byte) {
	var rbuf []byte
	frw := &fakeRw{
		_read: func(p []byte) (n int, err error) {
			if len(p) >= len(input) {
				ba := []byte(input)
				input = ""
				for i, b := range ba {
					p[i] = b
				}
				return len(ba), nil
			}

			ba := []byte(input[0:len(p)])
			input = input[len(p):]
			for i, b := range ba {
				p[i] = b
			}
			return len(ba), nil
		},
		_write: func(p []byte) (n int, err error) {
			rbuf = append(rbuf, p...)
			return len(p), nil
		},
		_close: func() error {
			return nil
		},
	}
	return frw, &rbuf
}

func TestValidateAuthentication(t *testing.T) {
	Convey("validateAuthentication is always successful", t, func() {
		c := &Session{}

		err, ok := c.validateAuthentication("OINK")
		So(err, ShouldBeNil)
		So(ok, ShouldBeTrue)

		err, ok = c.validateAuthentication("OINK", "arg1")
		So(err, ShouldBeNil)
		So(ok, ShouldBeTrue)

		err, ok = c.validateAuthentication("OINK", "arg1", "arg2")
		So(err, ShouldBeNil)
		So(ok, ShouldBeTrue)
	})
}

func TestValidateRecipient(t *testing.T) {
	Convey("validateRecipient is always successful", t, func() {
		c := &Session{}

		So(c.validateRecipient("OINK"), ShouldBeTrue)
		So(c.validateRecipient("foo@bar.mailhog"), ShouldBeTrue)
	})
}

func TestValidateSender(t *testing.T) {
	Convey("validateSender is always successful", t, func() {
		c := &Session{}

		So(c.validateSender("OINK"), ShouldBeTrue)
		So(c.validateSender("foo@bar.mailhog"), ShouldBeTrue)
	})
}
