package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func upTestServices() {
	response := `{
    "dirs": [
        "ru-en",
        "ru-pl",
        "ru-hu"
    ],
    "langs": {
        "ru": "русский",
        "en": "английский",
        "pl": "польский"
    }}`
	handlerTrLang := func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, response)
	}
	handlerTrLang(
		httptest.NewRecorder(),
		httptest.NewRequest("POST", urlMap["trLangs"], nil),
	)

	response = `["ru-ru","ru-en","ru-pl","ru-uk","ru-de","ru-fr","ru-es",
	"ru-it","ru-tr","en-ru","en-en","en-de","en-fr","en-es","en-it","en-tr",
	"pl-ru","uk-ru","de-ru","de-en","fr-ru","fr-en","es-ru","es-en",
	"it-ru","it-en","tr-ru","tr-en"]`
	handlerDictLang := func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, response)
	}
	handlerDictLang(
		httptest.NewRecorder(),
		httptest.NewRequest("POST", urlMap["dictLangs"], nil),
	)
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
	upTestServices()
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
	err := initLanguages(mainCtx)
	if err != nil {
		t.Errorf("init langs errors: %v", err)
	}
	//
	//ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	//	handlerEvent(ctx, w, r)
	//}))
	//defer ts.Close()
	//
	//res, err := http.Get(ts.URL)
	//if err != nil {
	//	t.Errorf("unexpected error: %v", err)
	//}
	//if s := res.StatusCode; s != http.StatusExpectationFailed {
	//	t.Errorf("wrong status: %v", s)
	//}
}
