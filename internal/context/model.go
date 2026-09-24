package context

type Context struct {
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Profile      string `json:"profile"`
	AutoSchedule bool   `json:"auto_schedule"`
	IsCurrent    bool   `json:"is_current"`
}
