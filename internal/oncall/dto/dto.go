package dto

type TeamCreateDTO struct {
	Name                      string `json:"name,omitempty"`
	Email                     string `json:"email,omitempty"`
	SchedulingTimezone        string `json:"scheduling_timezone,omitempty"`
	SlackChannel              string `json:"slack_channel,omitempty"`
	SlackChannelNotifications string `json:"slack_channel_notifications,omitempty"`
}

type UserCreateDTO struct {
	Name     string      `json:"name,omitempty"`
	FullName string      `json:"full_name,omitempty"`
	Contacts ContactsDTO `json:"contacts,omitempty"`
	TimeZone string      `json:"time_zone,omitempty"`
	PhotoURL string      `json:"photo_url,omitempty"`
}

type ContactsDTO struct {
	Call  string `json:"call,omitempty"`
	Email string `json:"email,omitempty"`
	SMS   string `json:"sms,omitempty"`
	Slack string `json:"slack,omitempty"`
}

type ScheduleDTO struct {
	Username      string `json:"user,omitempty"`
	Teamname      string `json:"team,omitempty"`
	Role          string `json:"role,omitempty"`
	StartTimeUnix int64  `json:"start,omitempty"`
	EndTimeUnix   int64  `json:"end,omitempty"`
}
