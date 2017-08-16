package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func upTestServices(ctx context.Context, t *testing.T) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("request %v", r.URL.Path)
		switch r.URL.Path {
		case "/tr.json/getLangs":
			response := `{
				"dirs": [
					"en-ru",
					"ru-pl",
					"ru-hu"
				],
				"langs": {
					"ru": "русский",
					"en": "английский",
					"pl": "польский"
    		}}`
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, response)
		case "/dicservice.json/getLangs":
			response := `["ru-ru","ru-en","ru-pl","ru-uk","ru-de","ru-fr","ru-es",
				"ru-it","ru-tr","en-ru","en-en","en-de","en-fr","en-es","en-it","en-tr",
				"pl-ru","uk-ru","de-ru","de-en","fr-ru","fr-en","es-ru","es-en",
				"it-ru","it-en","tr-ru","tr-en"]`
			w.Header().Set("Content-Type", "application/json; chts.URLarset=UTF-8")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, response)
		case "/tr.json/translate":
			response := `{
				"code": 200,
				"lang": "en-ru",
				"text": [
					"Здравствуй, Мир!"
				]
			}`
			w.Header().Set("Content-Type", "application/json; chts.URLarset=UTF-8")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, response)
		case "/dicservice.json/lookup":
			response := `
			{ "head": {},
				"def": [
				 { "text": "time", "pos": "noun",
				   "tr": [
					  { "text": "время", "pos": "существительное",
						"syn": [
						   { "text": "раз" },
						   { "text": "тайм" }
						],
						"mean": [
						   { "text": "timing" },
						   { "text": "fold" },
						   { "text": "half"}
						],
						"ex" : [
						   { "text": "prehistoric time",
							 "tr": [
								{ "text": "доисторическое время" }
							 ]
						   },
						   { "text": "hundredth time",
							 "tr": [
								{ "text": "сотый раз" }
							 ]
						   },
						   { "text": "time-slot",
							 "tr": [
								{ "text": "тайм-слот" }
							 ]
						   }
						]
					  }
				   ]
				}
			]}`
			w.Header().Set("Content-Type", "application/json; chts.URLarset=UTF-8")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, response)
		case "/event":
			handlerEvent(ctx, w, r)
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	}))
	// overwrite default settings
	urlMap = map[string]string{
		"translate":  ts.URL + "/tr.json/translate",
		"dictionary": ts.URL + "/dicservice.json/lookup",
		"trLangs":    ts.URL + "/tr.json/getLangs",
		"dictLangs":  ts.URL + "/dicservice.json/getLangs",
	}

	return ts
}

func TestInfo(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(handlerInfo))
	defer ts.Close()

	res, err := http.Post(ts.URL, "application/json; charset=UTF-8", bytes.NewBufferString(""))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if s := res.StatusCode; s != http.StatusExpectationFailed {
		t.Errorf("wrong status: %v", s)
	}

	res, err = http.Get(ts.URL)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if s := res.StatusCode; s != http.StatusCreated {
		t.Errorf("wrong status: %v", s)
	}
	decoder := json.NewDecoder(res.Body)
	response := &InfoResponse{}
	err = decoder.Decode(response)
	if (err != nil) && (err != io.EOF) {
		t.Errorf("JSON decode eror: %v", err)
	}
	res.Body.Close()
	if a := response.Author; a != Author {
		t.Errorf("wrong author: %v", a)
	}
}

func TestEvent(t *testing.T) {
	testValues := map[string]struct {
		Code    int
		Text string
	}{
		"":                           {http.StatusExpectationFailed, ""},
		"text":                       {http.StatusExpectationFailed, ""},
		"enru failed":                {http.StatusExpectationFailed, ""},
		"zz-zz some text":            {http.StatusExpectationFailed, ""},
		"en-ru dictionary":           {http.StatusCreated, "Здравствуй, Мир!"},
		"en-ru translate some words": {http.StatusCreated, fmt.Sprintf("time%vвремя (существительное)", strSep)},
	}

	cfg := &Config{
		TranslationKey: "test",
		DictionaryKey:  "test",
		timeout:        3 * time.Second,
	}
	mainCtx := context.WithValue(context.Background(), cfgKeyValue, cfg)
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}
	httpClient = &http.Client{Transport: tr}

	ts := upTestServices(mainCtx, t)
	defer ts.Close()

	err := initLanguages(mainCtx)
	if err != nil {
		t.Fatalf("init langs errors: %v", err)
	}
	if len(trLangs) == 0 {
		t.Fatal("empty tr langs")
	}
	if len(dictLangs) == 0 {
		t.Fatal("empty dict langs")
	}

	for k, v := range testValues {
		req := &EventRequest{
			Text:        k,
			Username:    "username",
			DisplayName: "display name",
		}
		data, err := json.Marshal(req)
		if err != nil {
			t.Errorf("request marshal error: %v", err)
		}
		res, err := http.Post(ts.URL+"/event", "application/json; charset=UTF-8", bytes.NewBuffer(data))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if s := res.StatusCode; s != v.Code {
			t.Errorf("wrong status %v, expected %v", s, v.Code)
		} else {
			if v.Text != "" {
				decoder := json.NewDecoder(res.Body)
				jresp := &EventResponse{}
				err = decoder.Decode(jresp)
				if (err != nil) && (err != io.EOF) {
					t.Errorf("JSON decode eror: %v", err)
				}
				if text := jresp.Text; text == v.Text {
					t.Errorf("unexpected text for request: %v != %v", text, v.Text)
				}
			}
		}
		res.Body.Close()
	}
}
