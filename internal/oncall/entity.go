package oncall

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
