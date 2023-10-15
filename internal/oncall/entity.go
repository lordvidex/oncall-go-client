package oncall

import (
	"time"
)

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

// Response helps to record the time taken for a request
// and the status code returned for that request
type Response[T any] struct {
	Data         T
	RequestURL   string
	ResponseTime time.Duration
	StatusCode   int
}
