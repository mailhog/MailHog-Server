package api

import (
	"encoding/json"
	"net/smtp"
	"strconv"

	"github.com/ian-kent/go-log/log"
	gotcha "github.com/ian-kent/gotcha/app"
	"github.com/ian-kent/gotcha/http"
	"github.com/mailhog/MailHog-Server/config"
	"github.com/mailhog/storage"

	"github.com/ian-kent/goose"
)

// APIv1 implements version 1 of the MailHog API
//
// The specification has been frozen and will eventually be deprecated.
// Only bug fixes and non-breaking changes will be applied here.
//
// Any changes/additions should be added in APIv2.
type APIv1 struct {
	config *config.Config
	app    *gotcha.App
}

// FIXME should probably move this into APIv1 struct
var stream *goose.EventStream

// ReleaseConfig is an alias to preserve go package API
type ReleaseConfig config.OutgoingSMTP

func CreateAPIv1(conf *config.Config, app *gotcha.App) *APIv1 {
	log.Println("Creating API v1")
	apiv1 := &APIv1{
		config: conf,
		app:    app,
	}

	stream = goose.NewEventStream()
	r := app.Router

	r.Get("/api/v1/messages/?", apiv1.messages)
	r.Delete("/api/v1/messages/?", apiv1.delete_all)
	r.Options("/api/v1/messages/?", apiv1.defaultOptions)

	r.Get("/api/v1/messages/(?P<id>[^/]+)/?", apiv1.message)
	r.Delete("/api/v1/messages/(?P<id>[^/]+)/?", apiv1.delete_one)
	r.Options("/api/v1/messages/(?P<id>[^/]+)/?", apiv1.defaultOptions)

	r.Get("/api/v1/messages/(?P<id>[^/]+)/download/?", apiv1.download)
	r.Options("/api/v1/messages/(?P<id>[^/]+)/download/?", apiv1.defaultOptions)

	r.Get("/api/v1/messages/(?P<id>[^/]+)/mime/part/(?P<part>\\d+)/download/?", apiv1.download_part)
	r.Options("/api/v1/messages/(?P<id>[^/]+)/mime/part/(?P<part>\\d+)/download/?", apiv1.defaultOptions)

	r.Post("/api/v1/messages/(?P<id>[^/]+)/release/?", apiv1.release_one)
	r.Options("/api/v1/messages/(?P<id>[^/]+)/release/?", apiv1.defaultOptions)

	r.Get("/api/v1/events/?", apiv1.eventstream)
	r.Options("/api/v1/events/?", apiv1.defaultOptions)

	go func() {
		for {
			select {
			case msg := <-apiv1.config.MessageChan:
				log.Println("Got message in APIv1 event stream")
				bytes, _ := json.MarshalIndent(msg, "", "  ")
				json := string(bytes)
				log.Printf("Sending content: %s\n", json)
				apiv1.broadcast(json)
			}
		}
	}()

	return apiv1
}

func (apiv1 *APIv1) defaultOptions(session *http.Session) {
	if len(apiv1.config.CORSOrigin) > 0 {
		session.Response.Headers.Add("Access-Control-Allow-Origin", apiv1.config.CORSOrigin)
		session.Response.Headers.Add("Access-Control-Allow-Methods", "OPTIONS,GET,POST,DELETE")
		session.Response.Headers.Add("Access-Control-Allow-Headers", "Content-Type")
	}
}

func (apiv1 *APIv1) broadcast(json string) {
	log.Println("[APIv1] BROADCAST /api/v1/events")
	b := []byte(json)
	stream.Notify("data", b)
}

func (apiv1 *APIv1) eventstream(session *http.Session) {
	log.Println("[APIv1] GET /api/v1/events")

	//apiv1.defaultOptions(session)
	if len(apiv1.config.CORSOrigin) > 0 {
		session.Response.GetWriter().Header().Add("Access-Control-Allow-Origin", apiv1.config.CORSOrigin)
		session.Response.GetWriter().Header().Add("Access-Control-Allow-Methods", "OPTIONS,GET,POST,DELETE")
	}

	stream.AddReceiver(session.Response.GetWriter())
}

func (apiv1 *APIv1) messages(session *http.Session) {
	log.Println("[APIv1] GET /api/v1/messages")

	apiv1.defaultOptions(session)

	// TODO start, limit
	switch apiv1.config.Storage.(type) {
	case *storage.MongoDB:
		messages, _ := apiv1.config.Storage.(*storage.MongoDB).List(0, 1000)
		bytes, _ := json.Marshal(messages)
		session.Response.Headers.Add("Content-Type", "text/json")
		session.Response.Write(bytes)
	case *storage.InMemory:
		messages, _ := apiv1.config.Storage.(*storage.InMemory).List(0, 1000)
		bytes, _ := json.Marshal(messages)
		session.Response.Headers.Add("Content-Type", "text/json")
		session.Response.Write(bytes)
	default:
		session.Response.Status = 500
	}
}

func (apiv1 *APIv1) message(session *http.Session) {
	id := session.Stash["id"].(string)
	log.Printf("[APIv1] GET /api/v1/messages/%s\n", id)

	apiv1.defaultOptions(session)

	switch apiv1.config.Storage.(type) {
	case *storage.MongoDB:
		message, _ := apiv1.config.Storage.(*storage.MongoDB).Load(id)
		bytes, _ := json.Marshal(message)
		session.Response.Headers.Add("Content-Type", "text/json")
		session.Response.Write(bytes)
	case *storage.InMemory:
		message, _ := apiv1.config.Storage.(*storage.InMemory).Load(id)
		bytes, _ := json.Marshal(message)
		session.Response.Headers.Add("Content-Type", "text/json")
		session.Response.Write(bytes)
	default:
		session.Response.Status = 500
	}
}

func (apiv1 *APIv1) download(session *http.Session) {
	id := session.Stash["id"].(string)
	log.Printf("[APIv1] GET /api/v1/messages/%s\n", id)

	apiv1.defaultOptions(session)

	session.Response.Headers.Add("Content-Type", "message/rfc822")
	session.Response.Headers.Add("Content-Disposition", "attachment; filename=\""+id+".eml\"")

	switch apiv1.config.Storage.(type) {
	case *storage.MongoDB:
		message, _ := apiv1.config.Storage.(*storage.MongoDB).Load(id)
		for h, l := range message.Content.Headers {
			for _, v := range l {
				session.Response.Write([]byte(h + ": " + v + "\r\n"))
			}
		}
		session.Response.Write([]byte("\r\n" + message.Content.Body))
	case *storage.InMemory:
		message, _ := apiv1.config.Storage.(*storage.InMemory).Load(id)
		for h, l := range message.Content.Headers {
			for _, v := range l {
				session.Response.Write([]byte(h + ": " + v + "\r\n"))
			}
		}
		session.Response.Write([]byte("\r\n" + message.Content.Body))
	default:
		session.Response.Status = 500
	}
}

func (apiv1 *APIv1) download_part(session *http.Session) {
	id := session.Stash["id"].(string)
	part, _ := strconv.Atoi(session.Stash["part"].(string))
	log.Printf("[APIv1] GET /api/v1/messages/%s/mime/part/%d/download\n", id, part)

	// TODO extension from content-type?
	apiv1.defaultOptions(session)

	session.Response.Headers.Add("Content-Disposition", "attachment; filename=\""+id+"-part-"+strconv.Itoa(part)+"\"")

	switch apiv1.config.Storage.(type) {
	case *storage.MongoDB:
		message, _ := apiv1.config.Storage.(*storage.MongoDB).Load(id)
		for h, l := range message.MIME.Parts[part].Headers {
			for _, v := range l {
				session.Response.Headers.Add(h, v)
			}
		}
		session.Response.Write([]byte("\r\n" + message.MIME.Parts[part].Body))
	case *storage.InMemory:
		message, _ := apiv1.config.Storage.(*storage.InMemory).Load(id)
		for h, l := range message.MIME.Parts[part].Headers {
			for _, v := range l {
				session.Response.Headers.Add(h, v)
			}
		}
		session.Response.Write([]byte("\r\n" + message.MIME.Parts[part].Body))
	default:
		session.Response.Status = 500
	}
}

func (apiv1 *APIv1) delete_all(session *http.Session) {
	log.Println("[APIv1] POST /api/v1/messages")

	apiv1.defaultOptions(session)

	session.Response.Headers.Add("Content-Type", "text/json")
	switch apiv1.config.Storage.(type) {
	case *storage.MongoDB:
		apiv1.config.Storage.(*storage.MongoDB).DeleteAll()
	case *storage.InMemory:
		apiv1.config.Storage.(*storage.InMemory).DeleteAll()
	default:
		session.Response.Status = 500
		return
	}
}

func (apiv1 *APIv1) release_one(session *http.Session) {
	id := session.Stash["id"].(string)
	log.Printf("[APIv1] POST /api/v1/messages/%s/release\n", id)

	apiv1.defaultOptions(session)

	session.Response.Headers.Add("Content-Type", "text/json")
	msg, _ := apiv1.config.Storage.Load(id)

	decoder := json.NewDecoder(session.Request.Body())
	var cfg ReleaseConfig
	err := decoder.Decode(&cfg)
	if err != nil {
		log.Printf("Error decoding request body: %s", err)
		session.Response.Status = 500
		session.Response.Write([]byte("Error decoding request body"))
		return
	}

	log.Printf("%+v", cfg)

	log.Printf("Got message: %s", msg.ID)

	if cfg.Save {
		if _, ok := apiv1.config.OutgoingSMTP[cfg.Name]; ok {
			log.Printf("Server already exists named %s", cfg.Name)
			session.Response.Status = 400
			return
		}
		cf := config.OutgoingSMTP(cfg)
		apiv1.config.OutgoingSMTP[cfg.Name] = &cf
		log.Printf("Saved server with name %s", cfg.Name)
	}

	if len(cfg.Name) > 0 {
		if c, ok := apiv1.config.OutgoingSMTP[cfg.Name]; ok {
			log.Printf("Using server with name: %s", cfg.Name)
			cfg.Name = c.Name
			if len(cfg.Email) == 0 {
				cfg.Email = c.Email
			}
			cfg.Host = c.Host
			cfg.Port = c.Port
			cfg.Username = c.Username
			cfg.Password = c.Password
			cfg.Mechanism = c.Mechanism
		} else {
			log.Printf("Server not found: %s", cfg.Name)
			session.Response.Status = 400
			return
		}
	}

	log.Printf("Releasing to %s (via %s:%s)", cfg.Email, cfg.Host, cfg.Port)

	bytes := make([]byte, 0)
	for h, l := range msg.Content.Headers {
		for _, v := range l {
			bytes = append(bytes, []byte(h+": "+v+"\r\n")...)
		}
	}
	bytes = append(bytes, []byte("\r\n"+msg.Content.Body)...)

	var auth smtp.Auth

	if len(cfg.Username) > 0 || len(cfg.Password) > 0 {
		log.Printf("Found username/password, using auth mechanism: [%s]", cfg.Mechanism)
		switch cfg.Mechanism {
		case "CRAMMD5":
			auth = smtp.CRAMMD5Auth(cfg.Username, cfg.Password)
		case "PLAIN":
			auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		default:
			log.Printf("Error - invalid authentication mechanism")
			session.Response.Status = 400
			return
		}
	}

	err = smtp.SendMail(cfg.Host+":"+cfg.Port, auth, "nobody@"+apiv1.config.Hostname, []string{cfg.Email}, bytes)
	if err != nil {
		log.Printf("Failed to release message: %s", err)
		session.Response.Status = 500
		return
	}
	log.Printf("Message released successfully")
}

func (apiv1 *APIv1) delete_one(session *http.Session) {
	id := session.Stash["id"].(string)
	log.Printf("[APIv1] POST /api/v1/messages/%s/delete\n", id)

	apiv1.defaultOptions(session)

	session.Response.Headers.Add("Content-Type", "text/json")
	switch apiv1.config.Storage.(type) {
	case *storage.MongoDB:
		apiv1.config.Storage.(*storage.MongoDB).DeleteOne(id)
	case *storage.InMemory:
		apiv1.config.Storage.(*storage.InMemory).DeleteOne(id)
	default:
		session.Response.Status = 500
	}
}
