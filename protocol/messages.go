package protocol

import (
	"encoding/json"
	"time"
)

const (
	CapabilityPluginEntries        = "plugin-entries"
	WindowToggleDesktopLyrics      = "lyrics-toggle"
	WindowToggleDesktopLyricsLock  = "lyrics-lock"
	HostAudioPreferences           = "audio-preferences"
	HostSaveDesktopLyrics          = "desktop-lyrics-save"
	HostSetDesktopLyricsColor      = "desktop-lyrics-color"
	HostSetStatusLyrics            = "status-lyrics-enabled"
	HostSetTaskbarLyricsFontSize   = "taskbar-lyrics-font-size"
	HostSetTaskbarLyricsFontFamily = "taskbar-lyrics-font-family"
	HostSetLyricsFontFamily        = "lyrics-font-family"
)

type PluginPrepare struct {
	Scope      Scope
	Facade     PluginSnapshot
	Settings   json.RawMessage
	Tray       bool
	Appearance string
	WindowIcon []byte
}

type PluginAction struct {
	Scope         Scope
	Action        string
	DesktopLyrics *DesktopLyricsPreferences
}

type PluginActionReply struct {
	Failure Failure
	Visible bool
}

type PrepareReply struct {
	Failure       Failure
	Configured    bool
	Preferences   AudioPreferences
	WindowActions []string
}

type PluginCall struct {
	Scope        Scope
	Action       string
	Track        Track
	Level        string
	Format       string
	Ref          string
	Size         int32
	DeadlineNano int64
	MediaStream  uint32
	PreviousURL  string
	FeedCursor   *QueueFeedCursor
}

type QueueFeedCursor struct {
	Page            uint64
	FirstID, LastID int64
}

type PluginMedia struct {
	Failure   Failure
	Stream    *PluginSource
	Original  string
	Trans     string
	Word      string
	CoverSize uint32
	Handled   bool
	Tracks    []Track
}

type PluginSource struct {
	URL, Type, Level string
	Size             int64
	Gain, Peak       float64
	Headers          map[string][]string
	Renewable        bool
	Digest           *ContentDigest
}

type PluginSnapshot struct {
	Current            *Track
	Queue              []Track
	Order              []int64
	QueueRevision      uint64
	Attempt            uint64
	UnchangedQueue     bool
	At                 int64
	Playing, Loading   bool
	Ready, Shuffle     bool
	Repeat             RepeatMode
	Radio, Error       string
	Position, Duration time.Duration
	Downloaded         float64
	DownloadKnown      bool
	Lyrics             []LyricLine
	LyricsLoaded       bool
	Levels             [SpectrumBands]float32
	LevelsLive         bool
	Stream             *PluginSource
	Volume             float64
	Presentation       Presentation
	Desktop            DesktopSettings
	Decoder            DecoderStatus
	Casting            CastingState
	EQ                 EQSettings
	EQAvailable        bool
	Search             []string
}

type PluginHostCall struct {
	Scope         Scope
	Action        string
	Tracks        []Track
	Index         int64
	Position      time.Duration
	Number        float64
	Flag          bool
	Text          string
	Feed          string
	Session       *PlaybackSession
	Settings      json.RawMessage
	DeadlineNano  int64
	QueueRevision uint64
	Attempt       uint64
	HasRevision   bool
	TaskID        uint64
	DesktopPatch  DesktopPreferencePatch
	DesktopLyrics *DesktopLyricsPreferences
	EQ            EQSettings
	View          Presentation
}

type PluginHostReply struct {
	Failure  Failure
	Snapshot PluginSnapshot
	Session  *PlaybackSession
	Audio    *PluginAudioDescription
	Task     *DesktopTask
}

type PluginAudioDescription struct {
	ID             uint32
	Rate, Channels int32
}
