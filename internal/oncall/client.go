package oncall

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/lordvidex/oncall-go-client/internal/oncall/dto"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

const (
	loginEndpoint = "/login"
	teamsEndpoint = "/api/v0/teams/"
	usersEndpoint = "/api/v0/users/"
)

var (
	ErrLoginFailed     = errors.New("login failed")
	ErrInvalidEndpoint = errors.New("invalid endpoint")
	ErrInvalidRequest  = errors.New("invalid request")
)

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
			With().Str("service", "oncall-client").Logger(),
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
	endpoint, err := url.JoinPath(c.oncallURL, loginEndpoint)
	if err != nil {
		return ErrInvalidEndpoint
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	data := url.Values{}
	data.Set("username", "root")
	data.Set("password", "root")
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		c.logger.Error().Caller().Err(err).Send()
		return ErrLoginFailed
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	res, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error().Caller().Err(err).Send()
		return ErrLoginFailed
	}
	defer res.Body.Close()

	m := make(map[string]string)
	json.NewDecoder(res.Body).Decode(&m)
	c.logger.Info().Int("status_code", res.StatusCode).Interface("response", m).Send()
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

func (c *Client) CreateSchedule(u User) error {
	c.logger.Info().Msgf("creating user: %s", u.Name)
	endpoint, err := url.JoinPath(c.oncallURL, usersEndpoint)
	if err != nil {
		return ErrInvalidEndpoint
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	data := dto.UserCreateDTO{
		Name:     u.Name,
		FullName: u.FullName,
		Contacts: dto.ContactsDTO{
			Call:  u.PhoneNumber,
			Email: u.Email,
		},
	}
	b, _ := json.Marshal(data)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(b))
	if err != nil {
		c.logger.Error().Caller().Err(err).Send()
		return ErrInvalidRequest
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-TOKEN", c.csrfToken)
	res, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error().Caller().Err(err).Msg("error creating user")
		return err
	}
	defer res.Body.Close()

	c.logger.Info().
		Int("status_code", res.StatusCode).Send()
	if res.StatusCode != http.StatusCreated {
		c.logger.Warn().Msg("status code is not 201")
	}
	return nil
}

func (c *Client) CreateUser(u User) error {
	c.logger.Info().Msgf("creating user: %s", u.Name)
	endpoint, err := url.JoinPath(c.oncallURL, usersEndpoint)
	if err != nil {
		return ErrInvalidEndpoint
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	data := dto.UserCreateDTO{
		Name:     u.Name,
		FullName: u.FullName,
		Contacts: dto.ContactsDTO{
			Call:  u.PhoneNumber,
			Email: u.Email,
		},
	}
	b, _ := json.Marshal(data)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(b))
	if err != nil {
		c.logger.Error().Caller().Err(err).Send()
		return ErrInvalidRequest
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-TOKEN", c.csrfToken)
	res, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error().Caller().Err(err).Msg("error creating user")
		return err
	}
	defer res.Body.Close()

	c.logger.Info().
		Int("status_code", res.StatusCode).Send()
	if res.StatusCode != http.StatusCreated {
		c.logger.Warn().Msg("status code is not 201")
	}
	return nil
}

func (c *Client) CreateTeam(t Team) error {
	c.logger.Info().Msgf("creating team: %s", t.Name)
	endpoint, err := url.JoinPath(c.oncallURL, teamsEndpoint)
	if err != nil {
		return ErrInvalidEndpoint
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
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
		c.logger.Error().Caller().Err(err).Send()
		return ErrInvalidRequest
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-TOKEN", c.csrfToken)
	res, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error().Caller().Err(err).Msg("error creating team")
		return err
	}
	defer res.Body.Close()

	c.logger.Info().
		Int("status_code", res.StatusCode).Send()
	if res.StatusCode != http.StatusCreated {
		c.logger.Warn().Msg("status code is not 201")
	}
	for _, u := range t.Users {
		err := c.CreateUser(u)
		if err != nil {
			c.logger.Warn().Err(err).
				Str("user_name", u.Name).Msg("error creating user")
		}
		err = c.AddUserToTeam(u.Name, t.Name)
		if err != nil {
			c.logger.Warn().Err(err).
				Str("user_name", u.Name).
				Str("team_name", t.Name).
				Msg("error adding user to team")

		}
	}
	return nil
}

func (c *Client) AddUserToTeam(username, teamname string) error {
	c.logger.Info().Msgf("adding user %s to team %s", username, teamname)
	endpoint, err := url.JoinPath(c.oncallURL, teamsEndpoint, teamname, "users")
	if err != nil {
		return ErrInvalidEndpoint
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	data := map[string]interface{}{
		"name": username,
	}
	b, _ := json.Marshal(data)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(b))
	if err != nil {
		c.logger.Error().Caller().Err(err).Send()
		return ErrInvalidRequest
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-TOKEN", c.csrfToken)
	res, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error().Caller().Err(err).Msg("error adding user to team")
		return err
	}
	defer res.Body.Close()

	c.logger.Info().
		Int("status_code", res.StatusCode).Send()
	if res.StatusCode != http.StatusCreated {
		c.logger.Warn().Msg("status code is not 201")
	}
	return nil
}

type Config struct {
	Teams []Team `yaml:"teams"`
}

type Team struct {
	Name               string `yaml:"name"`
	SchedulingTimezone string `yaml:"scheduling_timezone"`
	Email              string `yaml:"email"`
	SlackChannel       string `yaml:"slack_channel"`
	Users              []User `yaml:"users"`
}

type User struct {
	Name        string `yaml:"name"`
	FullName    string `yaml:"full_name"`
	PhoneNumber string `yaml:"phone_number"`
	Email       string `yaml:"email"`
	Schedule    []Duty `yaml:"duty"`
}

type Duty struct {
	Date string `yaml:"date"`
	Role string `yaml:"role"`
}
