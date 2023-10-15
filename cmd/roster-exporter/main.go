package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"

	"github.com/lordvidex/oncall-go-client/internal/oncall"
)

var (
	availableTeamMembersGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "oncall_avail_users",
			Help: "The number of current available team members that are in rotation and can be contacted for work",
		},
		[]string{"role", "team"},
	)
	requestDurationHist = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "oncall_http_request_duration_seconds",
			Help: "HTTP request duration in seconds made to the oncall server to gather metrics.",
		},
		[]string{"endpoint"},
	)
	statusCodeHist = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "oncall_http_status_code",
			Help: "http status codes when getting available team members in oncall",
		},
		[]string{"endpoint"},
	)
)

var (
	scrapeStr string
	oncallURL string
	port      int
)

func init() {
	flag.StringVar(&scrapeStr, "scrape-duration", "30s", "interval to update and fetch new metrics")
	flag.StringVar(&oncallURL, "oncall", "http://oncall-web:8080", "url of the oncall server")
	flag.IntVar(&port, "port", 9213, "port for hosting metrics")

	prometheus.MustRegister(availableTeamMembersGauge)
	prometheus.MustRegister(requestDurationHist)
	prometheus.MustRegister(statusCodeHist)
}

func main() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	logger := zerolog.New(zerolog.NewConsoleWriter())

	flag.Parse()
	scrapeDuration, err := time.ParseDuration(scrapeStr)
	if err != nil {
		log.Fatal("failed to parse scrape-duration")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := NewApp(logger, oncallURL, scrapeDuration)
	if err != nil {
		log.Fatalf("failed to create app exporter: %v", err)
	}
	go app.worker(ctx)
	http.Handle("/metrics", promhttp.Handler())

	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

type app struct {
	logger zerolog.Logger
	// oncall Client is used to make http calls to oncall server
	cl *oncall.Client
	// scrapeDuration is the amount of time before new metrics are scraped
	scrapeDuration time.Duration
	// reloginDuration is the time taken before client is relogged in, to refresh token
	reloginDuration time.Duration
}

func NewApp(logger zerolog.Logger, oncallURL string, scrapeDuration time.Duration) (*app, error) {
	cl, err := oncall.New(oncall.WithURL(oncallURL))
	if err != nil {
		return nil, err
	}
	a := &app{
		logger:          logger,
		scrapeDuration:  scrapeDuration,
		reloginDuration: time.Hour,
		cl:              cl,
	}
	if err = a.login(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *app) worker(ctx context.Context) {
	ticker := time.NewTicker(a.scrapeDuration)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.updateMetrics()
		case <-time.After(a.reloginDuration):
			a.login()
		}
	}
}

func (a *app) login() error {
	return a.cl.Login(context.Background())
}

func (a *app) updateMetrics() error {
	teamsResult, err := a.cl.GetTeams()
	if err != nil {
		return err
	}
	requestDurationHist.WithLabelValues(teamsResult.RequestURL).Observe(teamsResult.ResponseTime.Seconds())
	statusCodeHist.WithLabelValues(teamsResult.RequestURL).Observe(float64(teamsResult.StatusCode))

	var errs []error
	for _, team := range teamsResult.Data {
		data, err := a.cl.GetSummary(team)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		requestDurationHist.WithLabelValues(data.RequestURL).Observe(data.ResponseTime.Seconds())
		statusCodeHist.WithLabelValues(data.RequestURL).Observe(float64(data.StatusCode))
		for role, count := range data.Data {
			availableTeamMembersGauge.WithLabelValues(role, team).Set(float64(count))
		}
	}
	return errors.Join(errs...)
}
