package main

import (
	"context"
	"database/sql"
	"errors"
	"github.com/caarlos0/env/v9"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lordvidex/oncall-go-client/migrations"
	"github.com/m7shapan/njson"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

type config struct {
	DatabaseURL    string `env:"DATABASE_URL,notEmpty,unset"`
	PromURL        string `env:"PROMETHEUS_URL" envDefault:"http://oncall-prometheus:9090"`
	ScrapeInterval string `env:"SCRAPE_INTERVAL" envDefault:"1m"`
	LogLevel       string `env:"LOG_LEVEL"                   envDefault:"info"`
	MetricsFile    string `env:"METRICS_FILE,notEmpty"`
	HTTPPort       uint   `env:"HTTP_PORT"                   envDefault:"8080"`
}

func (a *app) promFetch(ctx context.Context, query string, defaultSLI float64) (value float64, err error) {
	queryParams := url.Values{
		"query": []string{query},
		"time":  []string{strconv.FormatInt(time.Now().Unix(), 10)},
	}
	endpoint, err := url.JoinPath(a.Cfg.PromURL, "api/v1/query")
	if err != nil {
		return defaultSLI, err
	}
	endpoint = endpoint + "?" + queryParams.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return defaultSLI, err
	}
	res, err := a.HTTPClient.Do(req)
	if err != nil {
		return defaultSLI, err
	}
	defer res.Body.Close()

	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return defaultSLI, err
	}

	var result = struct {
		Value string `njson:"data.result.0.value.1"`
	}{
		Value: "",
	}
	if err = njson.Unmarshal(bytes, &result); err != nil {
		return defaultSLI, err
	}
	if result.Value == "" {
		return defaultSLI, errors.New("empty response")
	}
	f, err := strconv.ParseFloat(result.Value, 64)
	if err != nil {
		return defaultSLI, err
	}
	return f, nil
}

type app struct {
	Cfg        config
	L          *zerolog.Logger
	HTTPClient *http.Client
	Metrics    []metric `yaml:"metrics"`
	pool       *pgxpool.Pool
}

type metric struct {
	SLO        float64 `yaml:"slo"`
	Alias      string  `yaml:"alias"`
	Metric     string  `yaml:"metric"`
	DefaultSLI float64 `yaml:"default_value"`
}

func (a *app) insertMetrics(ctx context.Context) error {
	for _, m := range a.Metrics {
		v, err := a.promFetch(ctx, m.Metric, m.DefaultSLI)
		logger := a.L.With().Str("metric", m.Metric).Logger()
		if err != nil {
			logger.Error().
				Err(err).
				Msg("error fetching metric")
		}
		err = a.insertDB(ctx, m.Alias, m.Metric, m.SLO, v, v > m.SLO)
		if err != nil {
			logger.Error().Err(err).Msg("error inserting to db")
			return err
		}
	}
	return nil
}

func (a *app) insertDB(ctx context.Context, alias, metric string, slo, value float64, slaMet bool) error {
	_, err := a.pool.Exec(
		ctx,
		`INSERT INTO sla_record (alias, metric, slo, value, met) 
VALUES ($1, $2, $3, $4, $5)`,
		alias,
		metric,
		slo,
		value,
		slaMet,
	)
	return err
}

func (a *app) loadMetrics() error {
	f, err := os.Open(a.Cfg.MetricsFile)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = yaml.NewDecoder(f).Decode(a); err != nil {
		return err
	}
	if len(a.Metrics) == 0 {
		return errors.New("no metrics loaded")
	}
	return nil
}

func (a *app) Start(ctx context.Context) error {
	if err := a.runMigrations(); err != nil {
		return err
	}

	dur, err := time.ParseDuration(a.Cfg.ScrapeInterval)
	if err != nil {
		return err
	}

	if err = a.loadMetrics(); err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, a.Cfg.DatabaseURL)
	if err != nil {
		return err
	}
	a.pool = pool

	ticker := time.NewTicker(dur)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err = a.insertMetrics(ctx); err != nil {
				a.L.Error().Err(err).Msg("error inserting metrics")
			}
		}
	}

}

func (a *app) runMigrations() error {
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("pgx"); err != nil {
		return err
	}
	db, err := sql.Open("pgx", a.Cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	if err = goose.Up(db, "."); err != nil {
		return err
	}
	return nil
}

func main() {
	var cfg config
	if err := env.Parse(&cfg); err != nil {
		log.Fatal(err)
	}

	lvl, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)
	logger := zerolog.New(zerolog.NewConsoleWriter())

	logger.Debug().Interface("config", cfg).Send()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := &app{
		Cfg:        cfg,
		L:          &logger,
		HTTPClient: http.DefaultClient,
	}
	if err := app.Start(ctx); err != nil {
		logger.Fatal().Err(err).Msg("app is stopping")
	}
}
