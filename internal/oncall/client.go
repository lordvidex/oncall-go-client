package oncall

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"

	"github.com/lordvidex/oncall-go-client/internal/oncall/dto"
)

const (
	loginEndpoint    = "/login"
	teamsEndpoint    = "/api/v0/teams/"
	usersEndpoint    = "/api/v0/users/"
	scheduleEndpoint = "/api/v0/events/"
)

var (
	ErrLoginFailed     = errors.New("login failed")
	ErrInvalidEndpoint = errors.New("invalid endpoint")
	ErrInvalidRequest  = errors.New("invalid request")
)

var defaultTimeout = time.Second * 10

// Client is the handler that makes request to oncall server for this client app
type Client struct {
	oncallURL string
	logger    zerolog.Logger

	httpClient *http.Client
	csrfToken  string
}

// Option is a callback for passing parameters to *Client
type Option func(*Client)

// WithURL sets the oncall server URL
func WithURL(oncallURL string) Option {
	return func(c *Client) {
		c.oncallURL = oncallURL
	}
}

// New creates a new oncall Client and logs in the client. An error can also be returned.
func New(opts ...Option) (*Client, error) {
	// create jar to store cookoo
	cookieJar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	client := &Client{
		oncallURL: "http://localhost:8080/",
		logger: zerolog.New(zerolog.NewConsoleWriter()).
			With().Timestamp().Str("service", "oncall-client").Logger(),
		httpClient: &http.Client{
			Jar: cookieJar,
		},
	}
	for _, opt := range opts {
		opt(client)
	}

	// login the client
	err = client.Login(context.Background())
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (c *Client) Login(ctx context.Context) error {
	logger := c.logger.With().Str("action", "login").Logger()
	endpoint, err := url.JoinPath(c.oncallURL, loginEndpoint)
	if err != nil {
		return ErrInvalidEndpoint
	}

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	data := url.Values{}
	data.Set("username", "root")
	data.Set("password", "root")
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		logger.Error().Caller().Err(err).Send()
		return ErrLoginFailed
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	res, err := c.httpClient.Do(req)
	if err != nil {
		logger.Error().Caller().Err(err).Send()
		return ErrLoginFailed
	}
	defer res.Body.Close()

	m := make(map[string]string)
	json.NewDecoder(res.Body).Decode(&m)
	logger.Info().Int("status_code", res.StatusCode).Interface("response", m).Send()
	c.csrfToken = m["csrf_token"]
	return nil
}

// LoadConfig reads a yaml file and creates the entities (teams, users and schedules) in this file
func (c *Client) LoadConfig(filename string) error {
	var config Config
	file, err := os.Open(filename)
	if err != nil {
		c.logger.Error().Caller().Err(err).Msg("failure opening file")
		return err
	}
	defer file.Close()

	err = yaml.NewDecoder(file).Decode(&config)
	if err != nil {
		c.logger.Err(err).Msgf("error decoding yaml file: %s", filename)
		return err
	}
	return c.createEntities(config)
}

func (c *Client) createEntities(config Config) error {
	for _, t := range config.Teams {
		err := c.CreateTeam(t)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) CreateSchedule(username, teamname string, schedule []Duty) error {
	logger := c.logger.With().
		Caller().
		Str("action", "create_schedule").
		Str("user", username).
		Str("team", teamname).
		Logger()

	logger.Info().Msg("creating schedule")

	var errs []error
	for _, duty := range schedule {
		err := c.addDayDuty(duty, username, teamname)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (c *Client) addDayDuty(duty Duty, username, teamname string) error {
	logger := c.logger.With().Str("action", "adding user duty").Logger()
	if duty.Date == "" {
		logger.Warn().
			Interface("duty", duty).
			Msg("empty date")
		return nil
	}

	endpoint, err := url.JoinPath(c.oncallURL, scheduleEndpoint)
	if err != nil {
		return ErrInvalidEndpoint
	}

	startTime, err := time.Parse("02/01/2006", duty.Date)
	if err != nil {
		logger.Err(err).
			Interface("duty", duty).
			Msg("error parsing time")
		return nil
	}
	endTime := startTime.Add(time.Hour * 24)

	if c.existsDayDuty(username, teamname, startTime.Unix(), endTime.Unix(), duty.Role) {
		logger.Info().
			Str("username", username).
			Str("teamname", teamname).
			Interface("duty", duty).
			Msg("duty already exists")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	data := dto.ScheduleDTO{
		Username:      username,
		Teamname:      teamname,
		Role:          duty.Role,
		StartTimeUnix: startTime.Unix(),
		EndTimeUnix:   endTime.Unix(),
	}
	b, _ := json.Marshal(data)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(b))
	if err != nil {
		logger.Error().Caller().Err(err).Send()
		return ErrInvalidRequest
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-TOKEN", c.csrfToken)
	res, err := c.httpClient.Do(req)
	if err != nil {
		logger.Error().Caller().Err(err).Msg("error creating event")
		return err
	}
	defer res.Body.Close()

	logger.Info().
		Int("status_code", res.StatusCode).Send()
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		logger.Warn().Bytes("data", b).Msg("status code is not 201")
	}
	return nil
}

func (c *Client) existsDayDuty(username, teamname string, start, end int64, role string) bool {
	endpoint, err := url.JoinPath(c.oncallURL, scheduleEndpoint)
	if err != nil {
		c.logger.Err(err).Caller().Msg("invalid endpoint")
		return false
	}
	req, _ := http.NewRequest(http.MethodGet, endpoint, nil)
	q := req.URL.Query()
	q.Add("user", username)
	q.Add("team", teamname)
	q.Add("start", strconv.FormatInt(start, 10))
	q.Add("end", strconv.FormatInt(end, 10))
	q.Add("role", role)

	req.URL.RawQuery = q.Encode()

	res, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Err(err).Msg("error checking for day duty")
		return false
	}
	defer res.Body.Close()
	var items []interface{}
	json.NewDecoder(res.Body).Decode(&items)
	return len(items) > 0
}

// CreateUser is a two-step HTTP request (POST) that first creates the username of the user
// and sends a PUT request to add the user's data
func (c *Client) CreateUser(u User) error {
	logger := c.logger.With().Str("user", u.Name).Str("action", "create_user").Logger()
	logger.Info().Msgf("creating user")
	endpoint, err := url.JoinPath(c.oncallURL, usersEndpoint)
	if err != nil {
		return ErrInvalidEndpoint
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	postData := map[string]interface{}{
		"name": u.Name,
	}
	b, _ := json.Marshal(postData)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(b))
	if err != nil {
		logger.Error().Caller().Err(err).Send()
		return ErrInvalidRequest
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-TOKEN", c.csrfToken)
	res, err := c.httpClient.Do(req)
	if err != nil {
		logger.Error().Caller().Err(err).Msg("error creating user")
		return err
	}
	defer res.Body.Close()

	logger.Info().
		Int("status_code", res.StatusCode).Send()
	if res.StatusCode != http.StatusCreated {
		logger.Warn().Msg("status code is not 201")
	}

	// PUT data
	logger.Info().Msg("updating user data")
	data := dto.UserCreateDTO{
		Name:     u.Name,
		FullName: u.FullName,
		Contacts: dto.ContactsDTO{
			Call:  u.PhoneNumber,
			Email: u.Email,
		},
	}
	b, _ = json.Marshal(data)
	endpoint, err = url.JoinPath(endpoint, u.Name)
	if err != nil {
		return ErrInvalidEndpoint
	}
	req, err = http.NewRequest(http.MethodPut, endpoint, bytes.NewReader(b))
	if err != nil {
		logger.Error().Caller().Err(err).Send()
		return ErrInvalidRequest
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-TOKEN", c.csrfToken)

	res, err = c.httpClient.Do(req)
	if err != nil {
		logger.Error().Caller().Err(err).Msg("error updating user data")
		return err
	}
	defer res.Body.Close()
	logger.Info().Int("status_code", res.StatusCode).Send()
	return nil
}

func (c *Client) CreateTeam(t Team) error {
	logger := c.logger.With().Str("action", "create_team").Logger()
	logger.Info().Msgf("creating team: %s", t.Name)
	endpoint, err := url.JoinPath(c.oncallURL, teamsEndpoint)
	if err != nil {
		return ErrInvalidEndpoint
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	data := dto.TeamCreateDTO{
		Name:                      t.Name,
		Email:                     t.Email,
		SchedulingTimezone:        t.SchedulingTimezone,
		SlackChannel:              t.SlackChannel,
		SlackChannelNotifications: t.SlackChannel + "-alert",
	}
	b, _ := json.Marshal(data)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(b))
	if err != nil {
		logger.Error().Caller().Err(err).Send()
		return ErrInvalidRequest
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-TOKEN", c.csrfToken)
	res, err := c.httpClient.Do(req)
	if err != nil {
		logger.Error().Caller().Err(err).Msg("error creating team")
		return err
	}
	defer res.Body.Close()

	logger.Info().
		Int("status_code", res.StatusCode).Send()
	if res.StatusCode != http.StatusCreated {
		logger.Warn().Msg("status code is not 201")
	}
	for _, u := range t.Users {
		logger := logger.With().
			Str("user_name", u.Name).
			Str("team_name", t.Name).
			Logger()
		err := c.CreateUser(u)
		if err != nil {
			logger.Warn().Err(err).
				Msg("error creating user")
		}
		err = c.AddUserToTeam(u.Name, t.Name)
		if err != nil {
			logger.Warn().Err(err).
				Msg("error adding user to team")
		}
		err = c.CreateSchedule(u.Name, t.Name, u.Schedule)
		if err != nil {
			logger.Warn().Err(err).
				Msg("error creating event")
		}
	}
	return nil
}

func (c *Client) GetTeams() (Response[[]string], error) {
	logger := c.logger.With().Str("action", "get_teams").Logger()
	endpoint, err := url.JoinPath(c.oncallURL, teamsEndpoint)
	if err != nil {
		return nil, ErrInvalidEndpoint
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		logger.Error().Caller().Err(err).Send()
		return nil, ErrInvalidRequest
	}

	var result Response[[]string]
	startTime := time.Now()

	// perform request
	res, err := c.httpClient.Do(req)
	if err != nil {
		logger.Error().Caller().Err(err).Msg("error fetching teams")
		return nil, err
	}
	defer res.Body.Close()

	// record metrics
	result.ResponseTime = time.Since(startTime)
	result.StatusCode = res.StatusCode
	logger.Info().Int("status_code", res.StatusCode).Send()

	if err = json.NewDecoder(res.Body).Decode(&result.Data); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetSummary(team string) (Response[map[string]int], error) {
	logger := c.logger.With().Str("action", "get current summary of roster").Logger()
	endpoint, err := url.JoinPath(c.oncallURL, teamsEndpoint, team, "summary")
	if err != nil {
		return nil, ErrInvalidEndpoint
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		logger.Error().Caller().Err(err).Send()
		return nil, ErrInvalidRequest
	}

	result := Response[map[string]int]
		Data: make(map[string]int),
	}
	startTime := time.Now()

	// perform request
	res, err := c.httpClient.Do(req)
	if err != nil {
		logger.Error().Caller().Err(err).Msg("error fetching summary")
		return nil, err
	}
	defer res.Body.Close()

	// record metrics
	result.ResponseTime = time.Since(startTime)
	result.StatusCode = res.StatusCode
	logger.Info().Int("status_code", res.StatusCode).Send()

	var response map[string]map[string][]any
	if err = json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, err
	}
	if _, ok := response["current"]; ok {
		currentSummary := response["current"]
		for k, v := range currentSummary {
			result.Data[k] = len(v)
		}
		return result, nil
	}
	return nil, nil
}

func (c *Client) AddUserToTeam(username, teamname string) error {
	logger := c.logger.With().Str("action", "add_user_to_team").Logger()
	logger.Info().Msgf("adding user %s to team %s", username, teamname)
	endpoint, err := url.JoinPath(c.oncallURL, teamsEndpoint, teamname, "users")
	if err != nil {
		return ErrInvalidEndpoint
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	data := map[string]interface{}{
		"name": username,
	}
	b, _ := json.Marshal(data)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(b))
	if err != nil {
		logger.Error().Caller().Err(err).Send()
		return ErrInvalidRequest
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-TOKEN", c.csrfToken)
	res, err := c.httpClient.Do(req)
	if err != nil {
		logger.Error().Caller().Err(err).Msg("error adding user to team")
		return err
	}
	defer res.Body.Close()

	logger.Info().
		Int("status_code", res.StatusCode).Send()
	if res.StatusCode != http.StatusCreated {
		logger.Warn().Msg("status code is not 201")
	}
	return nil
}
