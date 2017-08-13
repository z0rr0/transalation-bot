// Radio-t chat translation bot.
// It translates required sentences or words using Yandex translate API.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ctxKey string

// Config is API key storage.
type Config struct {
	Host           string `json:"host"`
	Port           uint   `json:"port"`
	TranslationKey string `json:"tkey"`
	DictionaryKey  string `json:"dkey"`
	TimeoutValue   uint   `json:"timeout"`
	timeout        time.Duration
}

// Translater is an interface to prepare JSON translation response.
type Translater interface {
	String() string
}

type Langer interface {
	Content() []string
}

// LangsList is a  list of dictionary's languages (from JSON response).
// It is sorted in ascending order.
type LangsList []string

// LangsListTr is a list of translation's languages (from JSON response).
type LangsListTr struct {
	Dirs  []string          `json:"dirs"`
	Langs map[string]string `json:"langs"`
}

// JSONTrDictExample is an internal type of JSONTrDict.
type JSONTrDictExample struct {
	Pos  string              `json:"pos"`
	Text string              `json:"text"`
	Tr   []map[string]string `json:"tr"`
}

// JSONTrDictItem is an internal type of JSONTrDict.
type JSONTrDictItem struct {
	Text string              `json:"text"`
	Pos  string              `json:"pos"`
	Syn  []map[string]string `json:"syn"`
	Mean []map[string]string `json:"mean"`
	Ex   []JSONTrDictExample `json:"ex"`
}

// JSONTrDictArticle is an internal type of JSONTrDict.
type JSONTrDictArticle struct {
	Pos  string           `json:"post"`
	Text string           `json:"text"`
	Ts   string           `json:"ts"`
	Gen  string           `json:"gen"`
	Tr   []JSONTrDictItem `json:"tr"`
}

// JSONTrDict is a type of a translation dictionary (from JSON response).
// It supports "Translater" interface.
type JSONTrDict struct {
	Head map[string]string   `json:"head"`
	Def  []JSONTrDictArticle `json:"def"`
}

// JSONTrResp is a type of a translation (from JSON response).
// It supports "Translater" interface.
type JSONTrResp struct {
	Code float64  `json:"code"`
	Lang string   `json:"lang"`
	Text []string `json:"text"`
}

// InfoResponse is http GET:/info JSON response.
type InfoResponse struct {
	Author   string   `json:"author"`
	Info     string   `json:"info"`
	Commands []string `json:"commands"`
}

// EventRequest is http POST:/event request.
type EventRequest struct {
	Text        string `json:"text"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

// EventResponse is http POSt:/event response.
type EventResponse struct {
	Text string `json:"text"`
	Bot  string `json:"bot"`
}

// Addr returns service's net address.
func (c *Config) Addr() string {
	return net.JoinHostPort(c.Host, fmt.Sprint(c.Port))
}

// Content is LangsList's implementation of Content method.
func (lg *LangsList) Content() []string {
	result := []string(*lg)
	sort.Strings(result)
	return result
}

// Content is LangsListTr's implementation of Content method.
func (lgt *LangsListTr) Content() []string {
	result := lgt.Dirs
	sort.Strings(result)
	return result
}

// String is an implementation of String() method for JSONTrResp pointer.
func (jstr *JSONTrResp) String() string {
	return strings.Join(jstr.Text, strSep)
}

// String is an implementation of String() method for JSONTrDict pointer.
// It returns a pretty formatted string.
func (jstrd *JSONTrDict) String() string {
	var (
		result, arResult []string
		txtResult        string
	)
	tabSym := fmt.Sprintf("%v  ", strSep)

	result = make([]string, len(jstrd.Def))
	for i, def := range jstrd.Def {
		ts := ""
		if def.Ts != "" {
			ts = fmt.Sprintf(" [%v] ", def.Ts)
		}
		txtResult = fmt.Sprintf("%v%v", def.Text, ts)
		if def.Pos != "" {
			txtResult += fmt.Sprintf("(%v)", def.Pos)
		}
		arResult = make([]string, len(def.Tr))
		for j, tr := range def.Tr {
			arResult[j] = fmt.Sprintf("%v (%v)", tr.Text, tr.Pos)
		}
		result[i] = fmt.Sprintf("%v%v%v", txtResult, strSep, strings.Join(arResult, tabSym))
	}
	return strings.Join(result, strSep)
}

// readConfig reads configuration file.
func readConfig(file string) (*Config, error) {
	if file == "" {
		file = filepath.Join(os.Getenv("HOME"), ConfigName)
	}
	_, err := os.Stat(file)
	if err != nil {
		return nil, err
	}
	jsondata, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	err = json.Unmarshal(jsondata, &cfg)
	if err != nil {
		return nil, err
	}
	if cfg.TimeoutValue != 0 {
		cfg.timeout = time.Duration(cfg.TimeoutValue) * time.Second
	} else {
		cfg.timeout = defaultTimeout
	}
	return cfg, nil
}

// request is a common method to send POST request and get []byte response.
func request(urlValue string, params *url.Values, timeout time.Duration) ([]byte, error) {
	var resp *http.Response
	req, err := http.NewRequest("POST", urlValue, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req = req.WithContext(ctx)

	ec := make(chan error)
	go func() {
		resp, err = httpClient.Do(req)
		ec <- err
		close(ec)
	}()
	select {
	case <-ctx.Done():
		<-ec // wait error "context deadline exceeded"
		return nil, fmt.Errorf("timed out (%v)", timeout)
	case err := <-ec:
		if err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wrong response code=%v", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// getLangs loads languages codes.
func getLangs(ctx context.Context, isTr bool) ([]string, error) {
	var (
		urlValue string
		result   Langer
		params   url.Values
	)
	c, ok := ctx.Value(cfgKeyValue).(*Config)
	if !ok {
		return nil, errors.New("configuration ctx not found")
	}
	if isTr {
		urlValue = urlMap["trLangs"]
		params = url.Values{"key": {c.TranslationKey}}
		result = &LangsListTr{}
	} else {
		urlValue = urlMap["dictLangs"]
		params = url.Values{"key": {c.DictionaryKey}, "ui": {"en"}}
		result = &LangsList{}
	}
	body, err := request(urlValue, &params, c.timeout)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, result)
	if err != nil {
		return nil, err
	}
	return result.Content(), nil
}

// initLanguages initializes languages arrays
func initLanguages(ctx context.Context) error {
	var err error
	trLangs, err = getLangs(ctx, true)
	if err != nil {
		return err
	}
	dictLangs, err = getLangs(ctx, true)
	if err != nil {
		return err
	}
	return nil
}

// isDirection checks - "direction" is language direction.
func isDirection(ctx context.Context, direction string, isTr bool) (bool, error) {
	var languages []string
	langsOnce.Do(func() {
		initLanguages(ctx)
	})
	if isTr {
		languages = trLangs
	} else {
		languages = dictLangs
	}
	if i := sort.SearchStrings(languages, direction); i < len(languages) && languages[i] == direction {
		return true, nil
	}
	return false, nil
}

// getTranslation returns translation result: "translate" or dictionary.
func getTranslation(ctx context.Context, isTr bool, direction, text string) (string, error) {
	var (
		urlValue string
		result   Translater
		params   url.Values
	)
	c, ok := ctx.Value(cfgKeyValue).(*Config)
	if !ok {
		return "", errors.New("configuration ctx not found")
	}
	if isTr {
		urlValue = urlMap["translate"]
		params = url.Values{
			"lang":   {direction},
			"text":   {text},
			"key":    {c.TranslationKey},
			"format": {"plain"},
		}
		result = &JSONTrResp{}
	} else {
		urlValue = urlMap["dictionary"]
		params = url.Values{
			"lang": {direction},
			"text": {text},
			"key":  {c.DictionaryKey},
		}
		result = &JSONTrDict{}
	}
	body, err := request(urlValue, &params, c.timeout)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(body, result)
	if err != nil {
		return "", err
	}
	return result.String(), nil
}

// Translate is a main translation method.
// It returns translated result and error value.
func Translate(ctx context.Context, text string) (string, error) {
	var isTr bool

	found := langDirect.FindAllStringIndex(text, 1)
	if len(found) == 0 {
		return "", nil
	}
	direction := strings.Trim(text[found[0][0]:found[0][1]], " ")
	parsed := strings.Trim(text[found[0][1]:], " ")

	// is it "translate" or "dictionary"
	elements := strings.SplitN(parsed, " ", 2)
	if len(elements) > 1 {
		isTr = true
	}
	ok, err := isDirection(ctx, direction, isTr)
	if err != nil {
		loggerInfo.Println("is not a direction")
		return "", err
	}
	if !ok {
		return "", nil
	}
	result, err := getTranslation(ctx, isTr, direction, parsed)
	if err != nil {
		return "", err
	}
	return result, nil
}
