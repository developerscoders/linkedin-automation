package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	LinkedIn  LinkedInConfig  `yaml:"linkedin"`
	Browser   BrowserConfig   `yaml:"browser"`
	Limits    LimitsConfig    `yaml:"limits"`
	Stealth   StealthConfig   `yaml:"stealth"`
	Schedule  ScheduleConfig  `yaml:"schedule"`
	Search    SearchConfig    `yaml:"search"`
	Messaging MessagingConfig `yaml:"messaging"`
	Storage   StorageConfig   `yaml:"storage"`
	Logging   LoggingConfig   `yaml:"logging"`
}

type LinkedInConfig struct {
	Email    string `yaml:"email"`
	Password string `yaml:"password"`
}

type BrowserConfig struct {
	Headless          bool           `yaml:"headless"`
	UserAgentRotation bool           `yaml:"user_agent_rotation"`
	ProxyURL          string         `yaml:"proxy_url"`
	Viewport          ViewportConfig `yaml:"viewport"`
	Timezone          string         `yaml:"timezone"`
	Language          string         `yaml:"language"`
}

type ViewportConfig struct {
	Width  int `yaml:"width"`
	Height int `yaml:"height"`
}

type LimitsConfig struct {
	DailyRequests         int `yaml:"daily_requests"`
	WeeklyRequests        int `yaml:"weekly_requests"`
	HourlyRequests        int `yaml:"hourly_requests"`
	HourlyMessages        int `yaml:"hourly_messages"`
	DailyMessages         int `yaml:"daily_messages"`
	MinActionDelaySeconds int `yaml:"min_action_delay_seconds"`
	MaxActionDelaySeconds int `yaml:"max_action_delay_seconds"`
	MaxContinuousHours    int `yaml:"max_continuous_hours"`
	MandatoryBreakMinutes int `yaml:"mandatory_break_minutes"`
}

type StealthConfig struct {
	Mouse    MouseConfig    `yaml:"mouse"`
	Timing   TimingConfig   `yaml:"timing"`
	Typing   TypingConfig   `yaml:"typing"`
	Scroll   ScrollConfig   `yaml:"scroll"`
	Behavior BehaviorConfig `yaml:"behavior"`
}

type MouseConfig struct {
	SpeedRange            []float64 `yaml:"speed_range"`
	OvershootRange        []float64 `yaml:"overshoot_range"`
	CorrectionProbability float64   `yaml:"correction_probability"`
}

type TimingConfig struct {
	ThinkTimeRange   []float64 `yaml:"think_time_range"`
	ReadWPMRange     []int     `yaml:"read_wpm_range"`
	ScrollThinkRange []float64 `yaml:"scroll_think_range"`
}

type TypingConfig struct {
	WPMRange        []int   `yaml:"wpm_range"`
	TypoProbability float64 `yaml:"typo_probability"`
}

type ScrollConfig struct {
	BackProbability  float64 `yaml:"back_probability"`
	HoverProbability float64 `yaml:"hover_probability"`
}

type BehaviorConfig struct {
	RandomHoverProbability    float64 `yaml:"random_hover_probability"`
	IdleMovementProbability   float64 `yaml:"idle_movement_probability"`
	ReadingPatternProbability float64 `yaml:"reading_pattern_probability"`
}

type ScheduleConfig struct {
	Timezone  string        `yaml:"timezone"`
	StartHour int           `yaml:"start_hour"`
	EndHour   int           `yaml:"end_hour"`
	WorkDays  []string      `yaml:"work_days"`
	Breaks    []BreakConfig `yaml:"breaks"`
}

type BreakConfig struct {
	StartHour       int `yaml:"start_hour"`
	EndHour         int `yaml:"end_hour"`
	DurationMinutes int `yaml:"duration_minutes"`
}

type SearchConfig struct {
	Keywords            []string `yaml:"keywords"` // Legacy, for backwards compatibility
	Jobs                []string `yaml:"jobs"`     // Job titles to search for connections
	Names               []string `yaml:"names"`    // Specific names to search and message
	MaxResultsPerSearch int      `yaml:"max_results_per_search"`
	MaxPages            int      `yaml:"max_pages"`
	CacheDurationHours  int      `yaml:"cache_duration_hours"`
}

type MessagingConfig struct {
	Templates []TemplateConfig `yaml:"templates"`
}

type TemplateConfig struct {
	Name    string `yaml:"name"`
	Content string `yaml:"content"`
}

type StorageConfig struct {
	MongoDB MongoDBConfig `yaml:"mongodb"`
}

type MongoDBConfig struct {
	URI            string `yaml:"uri"`
	Database       string `yaml:"database"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type LoggingConfig struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`
	Output     string `yaml:"output"`
	FilePath   string `yaml:"file_path"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
}

func Load(path string) (*Config, error) {
	config := &Config{}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(config); err != nil {
		return nil, err
	}

	// Override with environment variables
	if email := os.Getenv("LINKEDIN_EMAIL"); email != "" {
		config.LinkedIn.Email = email
	}
	if password := os.Getenv("LINKEDIN_PASSWORD"); password != "" {
		config.LinkedIn.Password = password
	}
	if uri := os.Getenv("MONGODB_URI"); uri != "" {
		config.Storage.MongoDB.URI = uri
	}
	if dbName := os.Getenv("MONGODB_DATABASE"); dbName != "" {
		config.Storage.MongoDB.Database = dbName
	}

	return config, nil
}
