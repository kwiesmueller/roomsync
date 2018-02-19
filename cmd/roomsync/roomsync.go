package main

import (
	"fmt"
	"runtime"

	"github.com/kwiesmueller/roomsync/pkg/hipchat"
	"github.com/kwiesmueller/roomsync/pkg/pipe"
	"github.com/kwiesmueller/roomsync/pkg/slack"
	"github.com/playnet-public/libs/log"

	"flag"

	raven "github.com/getsentry/raven-go"
	"github.com/kolide/kit/version"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	app    = "roomsync"
	appKey = "roomsync"
)

var (
	maxprocsPtr = flag.Int("maxprocs", runtime.NumCPU(), "max go procs")
	sentryDsn   = flag.String("sentrydsn", "", "sentry dsn key")
	dbgPtr      = flag.Bool("debug", false, "debug printing")
	versionPtr  = flag.Bool("version", true, "show or hide version info")

	slackToken   = flag.String("slackToken", "", "slack api token")
	slackChannel = flag.String("slackChannel", "", "slack api Channel")

	hipchatToken   = flag.String("hipchatToken", "", "hipchat api token")
	hipchatChannel = flag.String("hipchatChannel", "", "hipchat api channel")
	hipchatBaseURL = flag.String("hipchatBaseURL", "https://roomsync.cloud.play-net.org", "hipchat webhook baseurl")

	sentry *raven.Client
)

func main() {
	flag.Parse()

	if *versionPtr {
		fmt.Printf("-- %s --\n", app)
		version.PrintFull()
	}
	runtime.GOMAXPROCS(*maxprocsPtr)

	var zapFields []zapcore.Field
	// hide app and version information when debugging
	if !*dbgPtr {
		zapFields = []zapcore.Field{
			zap.String("app", appKey),
			zap.String("version", version.Version().Version),
		}
	}

	// prepare zap logging
	log := log.New(appKey, *sentryDsn, *dbgPtr).WithFields(zapFields...)
	defer log.Sync()
	log.Info("preparing")

	var err error

	// prepare sentry error logging
	sentry, err = raven.New(*sentryDsn)
	if err != nil {
		panic(err)
	}
	err = raven.SetDSN(*sentryDsn)
	if err != nil {
		panic(err)
	}

	// run main code
	log.Info("starting")
	sentryErr, sentryID := raven.CapturePanicAndWait(func() {
		if err := do(log); err != nil {
			log.Fatal("fatal error encountered", zap.Error(err))
			raven.CaptureErrorAndWait(err, map[string]string{"isFinal": "true"})
		}
	}, nil)
	if sentryErr != nil {
		log.Fatal("panic encountered", zap.String("sentryID", sentryID), zap.Error(sentryErr.(error)))
	}
	log.Info("finished")
}

func do(log *log.Logger) error {
	log.Info("creating pipes")
	s1 := slack.New(log, *slackToken, *slackChannel)
	//pipe2 := pipe.New(log, s2, s1)
	//go pipe2.Open()
	h := hipchat.New(log, *hipchatToken, *hipchatChannel, *hipchatBaseURL, "8080")

	pipe1 := pipe.New(log, s1, h)
	go pipe1.Open()
	pipe2 := pipe.New(log, h, s1)
	pipe2.Open()
	return nil
}
