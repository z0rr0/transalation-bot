// Radio-t chat translation bot.
// It translates required sentences or words using Yandex translate API.

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"time"
)

const (
	// Name is a program name
	Name = "translation-bot"
	// ConfigName is default configuration file name
	ConfigName = "config.json"
	// cfgKey is context key for configuration value
	cfgKeyValue ctxKey = "config"
	// interruptPrefix is constant prefix of interrupt signal
	interruptPrefix = "interrupt signal"
	// defaultTimeout is default configuration timeout (seconds)
	defaultTimeout = 3 * time.Second
	// userAgent is user-agent http header for external requests
	userAgent = "translation-bot"
	// strSep is a string separator
	strSep = "\n"
)

var (
	// Version is program version
	Version = ""
	// Revision is revision number
	Revision = ""
	// BuildDate is build date
	BuildDate = ""
	// GoVersion is runtime Go language version
	GoVersion = runtime.Version()

	// urlMap is services URLs
	urlMap = map[string]string{
		"translate":  "https://translate.yandex.net/api/v1.5/tr.json/translate",
		"dictionary": "https://dictionary.yandex.net/api/v1/dicservice.json/lookup",
		"trLangs":    "https://translate.yandex.net/api/v1.5/tr.json/getLangs",
		"dictLangs":  "https://dictionary.yandex.net/api/v1/dicservice.json/getLangs",
	}
	// langDirect is a regexp pattern to detect language direction.
	langDirect = regexp.MustCompile(`[a-z]{2,3}-[a-z]{2,3}`)

	// translation and dictionary languages storage
	trLangs   []string
	dictLangs []string
	// httpClient is base HTTP client struct
	httpClient *http.Client
	// internal loggers
	loggerError = log.New(os.Stderr, fmt.Sprintf("ERROR [%v]: ", Name),
		log.Ldate|log.Ltime|log.Lshortfile)
	loggerInfo = log.New(os.Stdout, fmt.Sprintf("INFO [%v]: ", Name),
		log.Ldate|log.Ltime|log.Lshortfile)
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			loggerError.Printf("abnormal termination [%v]: \n\t%v\n", Version, r)
		}
	}()
	version := flag.Bool("version", false, "show version")
	config := flag.String("config", ConfigName, "configuration file")
	flag.Parse()

	if *version {
		fmt.Printf("\tVersion: %v\n\tRevision: %v\n\tBuild date: %v\n\tGo version: %v\n",
			Version, Revision, BuildDate, GoVersion)
		return
	}
	cfg, err := readConfig(*config)
	if err != nil {
		loggerError.Panicf("configuration error: %v", err)
	}
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}
	httpClient = &http.Client{Transport: tr}

	fmt.Println(cfg)
}
