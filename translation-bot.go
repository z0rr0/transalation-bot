// Radio-t chat translation bot.
// It translates required sentences or words using Yandex translate API.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"syscall"
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
	// Author is author email
	Author = "thebestzorro@yandex.ru"

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

// interrupt catches custom signals.
func interrupt(errc chan error) {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	errc <- fmt.Errorf("%v %v", interruptPrefix, <-c)
}

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
	mainCtx := context.WithValue(context.Background(), cfgKeyValue, cfg)
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}
	httpClient = &http.Client{Transport: tr}
	err = initLanguages(mainCtx)
	if err != nil {
		loggerError.Panicf("no languages: %v", err)
	}
	// server
	server := &http.Server{
		Addr:           cfg.Addr(),
		Handler:        http.DefaultServeMux,
		MaxHeaderBytes: 1 << 20, // 1MB
		ErrorLog:       loggerError,
	}
	// handlers
	http.HandleFunc("/info", handlerInfo)
	http.HandleFunc("/event", func(w http.ResponseWriter, r *http.Request) {
		handlerEvent(mainCtx, w, r)
	})
	errCh := make(chan error)
	go interrupt(errCh)
	go func() {
		errCh <- server.ListenAndServe()
	}()
	loggerInfo.Printf("running: version=%v [%v %v]\nListen: %v\n\n",
		Version, GoVersion, Revision, server.Addr)
	err = <-errCh
	loggerInfo.Printf("termination: %v [%v] reason: %+v\n", Version, Revision, err)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	if msg := err.Error(); strings.HasPrefix(msg, interruptPrefix) {
		loggerInfo.Println("graceful shutdown")
		if err := server.Shutdown(ctx); err != nil {
			loggerError.Printf("graceful shutdown error: %v\n", err)
		}
	}
}
