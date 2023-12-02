package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"

	"github.com/lordvidex/oncall-go-client/internal/oncall"
)

var (
	// user
	createUserScenarioTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "prober_create_user_scenario_total",
		Help: "Total count of runs the create user scenario to oncall API",
	})
	createUserScenarioSuccess = promauto.NewCounter(prometheus.CounterOpts{
		Name: "prober_create_user_scenario_success_total",
		Help: "Total count of success runs the create user scenario to oncall API",
	})
	createUserScenarioDurationSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "prober_create_user_scenario_duration_seconds",
		Help: "Total duration of runs the create user scenario to oncall API",
	})

	// team
	createTeamScenarioTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "prober_create_team_scenario_total",
		Help: "Total count of runs the create team scenario to oncall API",
	})
	createTeamScenarioSuccess = promauto.NewCounter(prometheus.CounterOpts{
		Name: "prober_create_team_scenario_success_total",
		Help: "Total count of success runs the create team scenario to oncall API",
	})
	createTeamScenarioDurationSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "prober_create_team_scenario_duration_seconds",
		Help: "Total duration of runs the create team scenario to oncall API",
	})

	// add user to team
	addUserToTeamScenarioTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "prober_add_user_to_team_scenario_total",
		Help: "Total count of runs the create team scenario to oncall API",
	})
	addUserToTeamScenarioSuccess = promauto.NewCounter(prometheus.CounterOpts{
		Name: "prober_add_user_to_team_scenario_success_total",
		Help: "Total count of success runs to add user to team scenario to oncall API",
	})
	addUserToTeamScenarioDurationSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "prober_add_user_to_team_scenario_duration_seconds",
		Help: "Total duration of runs to add user to team scenario to oncall API",
	})
)

var (
	filename  string
	scrapeStr string
	oncallURL string
	port      int
	silent    bool
)

func init() {
	flag.StringVar(&filename, "f", "", "yaml config file to read probe data from")

	flag.StringVar(&scrapeStr, "scrape-duration", "60s", "interval to update and fetch new metrics")
	flag.StringVar(&oncallURL, "oncall", "http://oncall-web:8080", "url of the oncall server")
	flag.IntVar(&port, "port", 8080, "port for hosting metrics.. Prober hosts metrics on /probe")
	flag.BoolVar(&silent, "silent", false, "if true, logs are not printed for oncall client")
}

func main() {
	flag.Parse()
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	logger := zerolog.New(zerolog.NewConsoleWriter())

	if filename == "" {
		logger.Fatal().Msg("filename must be provided")
	}

	scrapeDuration, err := time.ParseDuration(scrapeStr)
	if err != nil {
		log.Fatal("failed to parse scrape-duration")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := NewApp(logger, oncallURL, scrapeDuration)
	if err != nil {
		log.Fatalf("failed to create prober: %v", err)
	}
	go app.worker(ctx)

	http.Handle("/probe", promhttp.Handler())
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

type app struct {
	logger zerolog.Logger
	// oncall Client is used to make http calls to oncall server
	cl *oncall.Client
	// oncall Config contains the test data to run SLA probe checks
	config oncall.Config
	// scrapeDuration is the amount of time before new metrics are scraped
	scrapeDuration time.Duration
	// reloginDuration is the time taken before client is relogged in, to refresh token
	reloginDuration time.Duration
}

func NewApp(logger zerolog.Logger, oncallURL string, scrapeDuration time.Duration) (*app, error) {
	cfg, err := oncall.LoadConfig(filename)
	if err != nil {
		return nil, err
	}

	opts := []oncall.Option{oncall.WithURL(oncallURL)}
	if silent {
		opts = append(opts, oncall.WithLogger(zerolog.Nop()))
	}
	cl, err := oncall.New(opts...)
	if err != nil {
		return nil, err
	}
	return &app{
		logger:          logger,
		scrapeDuration:  scrapeDuration,
		reloginDuration: time.Hour,
		config:          cfg,
		cl:              cl,
	}, nil
}

func (a *app) login() error {
	return a.cl.Login(context.Background())
}

func (a *app) worker(ctx context.Context) {
	ticker := time.NewTicker(a.scrapeDuration)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.runScenarios()
		case <-time.After(a.reloginDuration):
			a.login()
		}
	}
}

func (a *app) runScenarios() error {
	stats, err := a.cl.CreateEntities(a.config)
	defer a.cl.DeleteEntities(a.config)
	if err != nil {
		a.logger.Warn().Err(err).Msg("entities error")
	}

	// teams
	for _, tt := range a.config.Teams {
		createTeamScenarioTotal.Inc()
		teamStat, ok := stats[tt.Name]
		if !ok {
			createTeamScenarioSuccess.Add(0)
			continue
		}
		createTeamScenarioDurationSeconds.Set(float64(teamStat.Response.ResponseTime.Seconds()))
		if teamStat.Response.StatusCode <= 201 {
			createUserScenarioSuccess.Inc()
		} else {
			createUserScenarioSuccess.Add(0)
		}

		// users
		for _, u := range tt.Users {
			createUserScenarioTotal.Inc()
			addUserToTeamScenarioTotal.Inc()

			createRes, ok := teamStat.UserCreateResponses[u.Name]
			if !ok {
				createUserScenarioSuccess.Add(0)
				continue
			}
			if createRes.StatusCode != 0 && createRes.StatusCode <= 201 {
				createUserScenarioSuccess.Inc()
				createUserScenarioDurationSeconds.Set(float64(createRes.ResponseTime.Seconds()))
			} else {
				createUserScenarioSuccess.Add(0)
			}

			addRes, ok := teamStat.UserAddToTeamResponses[u.Name]
			if !ok {
				addUserToTeamScenarioSuccess.Add(0)
				continue
			}
			if addRes.StatusCode != 0 && addRes.StatusCode <= 201 {
				addUserToTeamScenarioSuccess.Inc()
				addUserToTeamScenarioDurationSeconds.Set(float64(addRes.ResponseTime.Seconds()))
			} else {
				addUserToTeamScenarioSuccess.Add(0)
			}
		}
	}
	return nil
}
