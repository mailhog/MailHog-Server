package api

import (
	"encoding/json"
	"strconv"

	"github.com/ian-kent/go-log/log"
	gotcha "github.com/ian-kent/gotcha/app"
	"github.com/ian-kent/gotcha/http"
	"github.com/mailhog/MailHog-Server/config"
	"github.com/mailhog/data"

	"github.com/ian-kent/goose"
)

// APIv2 implements version 2 of the MailHog API
//
// It is currently experimental and may change in future releases.
// Use APIv1 for guaranteed compatibility.
type APIv2 struct {
	config *config.Config
	app    *gotcha.App
}

func CreateAPIv2(conf *config.Config, app *gotcha.App) *APIv2 {
	log.Println("Creating API v2")
	apiv2 := &APIv2{
		config: conf,
		app:    app,
	}

	stream = goose.NewEventStream()
	r := app.Router

	r.Get("/api/v2/messages/?", apiv2.messages)
	r.Options("/api/v2/messages/?", apiv2.defaultOptions)

	r.Get("/api/v2/search/?", apiv2.search)
	r.Options("/api/v2/search/?", apiv2.defaultOptions)

	return apiv2
}

func (apiv2 *APIv2) defaultOptions(session *http.Session) {
	if len(apiv2.config.CORSOrigin) > 0 {
		session.Response.Headers.Add("Access-Control-Allow-Origin", apiv2.config.CORSOrigin)
		session.Response.Headers.Add("Access-Control-Allow-Methods", "OPTIONS,GET,POST,DELETE")
		session.Response.Headers.Add("Access-Control-Allow-Headers", "Content-Type")
	}
}

type messagesResult struct {
	Total int            `json:"total"`
	Count int            `json:"count"`
	Start int            `json:"start"`
	Items []data.Message `json:"items"`
}

func (apiv2 *APIv2) getStartLimit(session *http.Session) (start, limit int) {
	start = 0
	limit = 50

	s := session.Request.URL.Query().Get("start")
	if n, e := strconv.ParseInt(s, 10, 64); e == nil && n > 0 {
		start = int(n)
	}

	l := session.Request.URL.Query().Get("limit")
	if n, e := strconv.ParseInt(l, 10, 64); e == nil && n > 0 {
		if n > 250 {
			n = 250
		}
		limit = int(n)
	}

	return
}

func (apiv2 *APIv2) messages(session *http.Session) {
	log.Println("[APIv2] GET /api/v2/messages")

	apiv2.defaultOptions(session)

	start, limit := apiv2.getStartLimit(session)

	var res messagesResult

	messages, _ := apiv2.config.Storage.List(start, limit)

	res.Count = len([]data.Message(*messages))
	res.Start = start
	res.Items = []data.Message(*messages)
	res.Total = apiv2.config.Storage.Count()

	bytes, _ := json.Marshal(res)
	session.Response.Headers.Add("Content-Type", "text/json")
	session.Response.Write(bytes)
}

func (apiv2 *APIv2) search(session *http.Session) {
	log.Println("[APIv2] GET /api/v2/search")

	apiv2.defaultOptions(session)

	start, limit := apiv2.getStartLimit(session)

	kind := session.Request.URL.Query().Get("kind")
	if kind != "from" && kind != "to" && kind != "containing" {
		session.Response.Status = 400
		return
	}

	query := session.Request.URL.Query().Get("query")
	if len(query) == 0 {
		session.Response.Status = 400
		return
	}

	var res messagesResult

	messages, total, _ := apiv2.config.Storage.Search(kind, query, start, limit)

	res.Count = len([]data.Message(*messages))
	res.Start = start
	res.Items = []data.Message(*messages)
	res.Total = total

	bytes, _ := json.Marshal(res)
	session.Response.Headers.Add("Content-Type", "text/json")
	session.Response.Write(bytes)
}
