package main

import (
	_ "expvar"
	"flag"
	"fmt"
	"koding/tools/config"
	// Imported for side-effect of handling /debug/vars.
	"github.com/koding/logging"
	"github.com/koding/rabbitmq"
	// "koding/tools/logger"
	_ "net/http/pprof" // Imported for side-effect of handling /debug/pprof.
	"os"
	"os/signal"
	"socialapi/db"
	"socialapi/models"
	"socialapi/workers/api/handlers"
	"strings"
	"syscall"

	"github.com/koding/bongo"
	"github.com/koding/broker"
	"github.com/rcrowley/go-tigertonic"
)

var (
	Bongo       *bongo.Bongo
	log         = logging.NewLogger("FollowingFeedWorker")
	cert        = flag.String("cert", "", "certificate pathname")
	key         = flag.String("key", "", "private key pathname")
	flagConfig  = flag.String("config", "", "pathname of JSON configuration file")
	listen      = flag.String("listen", "127.0.0.1:8000", "listen address")
	flagProfile = flag.String("c", "", "Configuration profile from file")
	flagDebug   = flag.Bool("d", false, "Debug mode")
	conf        *config.Config

	hMux       tigertonic.HostServeMux
	mux, nsMux *tigertonic.TrieServeMux
)

type context struct {
	Username string
}

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: example [-cert=<cert>] [-key=<key>] [-config=<config>] [-listen=<listen>]")
		flag.PrintDefaults()
	}
	mux = tigertonic.NewTrieServeMux()
	mux = handlers.Inject(mux)
}

func setLogLevel() {
	var logLevel logging.Level

	if *flagDebug {
		logLevel = logging.DEBUG
	} else {
		logLevel = logging.INFO
	}
	log.SetLevel(logLevel)
}

func main() {
	flag.Parse()
	if *flagProfile == "" {
		log.Fatal("Please define config file with -c")
	}
	conf = config.MustConfig(*flagProfile)
	setLogLevel()

	// Example of parsing a configuration file.
	// c := &config.Config{}
	// if err := tigertonic.Configure(*flagConfig, c); nil != err {
	// 	log.Fatal(err)
	// }

	server := newServer()
	// Example use of server.Close and server.Wait to stop gracefully.
	go listener(server)

	// panics if not successful
	Bongo = helper.MustInitBongo(conf)

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

	log.Info("Recieved %v", <-ch)
	shutdown()
}

func shutdown() {
	log.Info("Closing connections")
	// do not forgot to close the bongo connection
	Bongo.Close()

	// shutdown server
	server.Close()
}

func newServer() *tigertonic.Server {
	return tigertonic.NewServer(
		*listen,
		tigertonic.CountedByStatus(
			tigertonic.Logged(
				tigertonic.WithContext(mux, context{}),
				func(s string) string {
					return strings.Replace(s, "SECRET", "REDACTED", -1)
				},
			),
			"http",
			nil,
		),
	)
}

func listener(server *tigertonic.Server) {
	var err error
	if "" != *cert && "" != *key {
		err = server.ListenAndServeTLS(*cert, *key)
	} else {
		err = server.ListenAndServe()
	}
	if nil != err {
		panic(err)
	}
}
