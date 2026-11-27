package protocol

import "time"

type Artist struct {
	ID   int64
	Name string
}

type Album struct {
	ID           int64
	Name, PicURL string
}

// PlatformTrack identifies an item in its platform instance. Ref is the
// original platform ID and may be a non-numeric string.
type PlatformTrack struct {
	Source, Ref, Format string
	Cue, Fav            bool
	DiscNo, TrackNo     int64
}

type Track struct {
	PlatformTrack
	ID          int64
	Name        string
	Artists     []Artist
	Album       Album
	Duration    int64 // milliseconds
	Fee         int64
	Alias       []string
	Reason      string
	Privilege   *Privilege
	ProgramID   int64
	PodcastID   int64
	Description string
}

type Privilege struct {
	ID, St, Pl int64
}

type ContentDigest struct {
	Algorithm string
	Sum       []byte
}

const SpectrumBands = 24

type RepeatMode int64

const (
	RepeatOff RepeatMode = iota
	RepeatAll
	RepeatOne
)

type PlaybackSession struct {
	Queue    []Track
	Order    []int64
	At       int64
	Shuffle  bool
	Repeat   RepeatMode
	Position int64 // milliseconds
	Radio    string
}

type Presentation struct {
	Smoke, Fog, LyricsStage, StageSparks bool
	StageStyle                           string
}

type CastingDevice struct {
	ID, Name string
}

type CastingState struct {
	Available, Sending, Scanning bool
	Devices                      []CastingDevice
	Active                       *CastingDevice
	ScanTask, OutputTask         DesktopTask
}

type DesktopPreferences struct {
	Appearance, AppIcon                            string
	CacheMB                                        int64
	Notch, NotchActivity, NotchVirtual, Smoke, Fog bool
	UseMPV, MPVExclusive, MPVGapless               bool
	MPVAudioDevice                                 string
}

type DesktopSettings struct {
	DesktopLyricsColorAvailable   bool
	DesktopLyricsColor            string
	StatusLyricsAvailable         bool
	StatusLyricsEnabled           bool
	TaskbarLyricsFontSize         int32
	TaskbarLyricsFontFamily       string
	LyricsFontAvailable           bool
	LyricsFontFamily              string
	Preferences                   DesktopPreferences
	CacheUsed                     int64
	NotchAvailable, Updates       bool
	UpdateChecks, UpdateDownloads bool
	LastUpdateCheck               time.Time
	CacheTask                     DesktopTask
}

type DesktopLyricsPreferences struct {
	Locked, ShowTranslation bool
	FontSize                float32
	Color, X, Y             int64
	PositionSet             bool
}

type AudioDevice struct {
	Name, Description string
}

type DecoderStatus struct {
	Ready, Downloaded, Downloading bool
	SystemPath                     string
	Done, Total                    int64
	Devices                        []AudioDevice
	DownloadTask, DeleteTask       DesktopTask
}

type EQSettings struct {
	Enabled                bool
	Type                   string
	Preamp                 float64
	Bands                  []float64
	Preset                 string
	AutoEQPath, AutoEQName string
}

type LyricLine struct {
	At, End           time.Duration
	Text, Translation string
	Words             []LyricWord
	Timed             bool
}

type LyricWord struct {
	Text       string
	Start, End time.Duration
}

type AudioPreferences struct {
	Quality        string
	Normalize      bool
	ReplayGainMode string
}
