package desktop

import (
	"context"
	"encoding/json"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"github.com/waygu-music-plugin/sdk/bridge"
	"github.com/waygu-music-plugin/sdk/pluginapi"
	"github.com/waygu-music-plugin/sdk/pluginapi/artwork"
	"github.com/waygu-music-plugin/sdk/pluginapi/eq"
	"github.com/waygu-music-plugin/sdk/pluginapi/lyrics"
	"github.com/waygu-music-plugin/sdk/pluginapi/network"
	"github.com/waygu-music-plugin/sdk/protocol"
)

// The child reads snapshots locally while drawing. Commands use an ordered
// queue so an RPC round trip never blocks the UI event loop.
type pluginClientServices struct {
	child      *pluginChild
	state      bridge.PluginSnapshot
	updated    time.Time
	commands   chan bridge.PluginHostCall
	art        *artwork.Cache
	window     atomic.Pointer[mygo.Window]
	toast      string
	lyrics     LyricsWindow
	pagination map[string]*pluginFeed
	nextPage   uint64
}

type pluginFeed struct {
	next pluginapi.QueueFeed
}

func newPluginClientServices(child *pluginChild, state bridge.PluginSnapshot) *pluginClientServices {
	s := &pluginClientServices{
		child: child, state: state, updated: time.Now(), commands: make(chan bridge.PluginHostCall, 128),
		pagination: make(map[string]*pluginFeed),
	}
	s.lyrics.Configure(pluginapi.DefaultDesktopLyricsPreferences(), func(prefs pluginapi.DesktopLyricsPreferences) {
		s.send(bridge.PluginHostCall{Action: protocol.HostSaveDesktopLyrics, DesktopLyrics: bridge.WireDesktopLyricsPrefs(prefs)})
	})
	s.art = artwork.NewCache(s.Post)
	go s.writeCommands()
	return s
}

func (s *pluginClientServices) writeCommands() {
	for {
		select {
		case <-s.child.ctx.Done():
			return
		case cmd := <-s.commands:
			ctx, cancel := context.WithTimeout(s.child.ctx, 5*time.Second)
			deadline, _ := ctx.Deadline()
			cmd.DeadlineNano = deadline.UnixNano()
			var reply bridge.PluginHostReply
			err := s.child.callHost(ctx, cmd, &reply)
			cancel()
			if err == nil {
				err = reply.Failure.Err()
			}
			if err != nil && s.child.ctx.Err() == nil {
				s.Post(func() { s.Toast(err.Error()) })
			}
		}
	}
}

func (s *pluginClientServices) send(in bridge.PluginHostCall) {
	in.Scope = s.child.scope
	select {
	case s.commands <- in:
	case <-s.child.ctx.Done():
	default:
		s.Post(func() { s.Toast("宿主暂时无法接收插件请求") })
	}
}

// PluginEntrySwitcher is an optional service for packages that provide more than one connection type.
type PluginEntrySwitcher interface {
	CanSwitchPluginEntry() bool
	SwitchPluginEntry(string)
}

func (s *pluginClientServices) CanSwitchPluginEntry() bool { return s.child.entrySwitch }

func (s *pluginClientServices) SwitchPluginEntry(id string) {
	if s.child.entrySwitch {
		s.send(bridge.PluginHostCall{Action: "plugin-entry", Text: id})
	}
}

func (s *pluginClientServices) action(name string) { s.send(bridge.PluginHostCall{Action: name}) }

func (s *pluginClientServices) Post(fn func()) {
	if win := s.window.Load(); win != nil {
		win.Update(func() {
			if s.child.ctx.Err() == nil {
				fn()
				s.Invalidate()
			}
		})
	}
}

func (s *pluginClientServices) Invalidate() {
	if s.child.main != nil {
		s.child.main.Invalidate()
	}
	for _, window := range s.child.aux {
		window.Invalidate()
	}
	s.lyrics.Invalidate()
}

func (s *pluginClientServices) ShowMain() { s.child.showMain() }
func (s *pluginClientServices) Toast(text string) {
	s.toast = text
	s.Invalidate()
}
func (s *pluginClientServices) TakeToast() string {
	text := s.toast
	s.toast = ""
	return text
}

func (s *pluginClientServices) OpenSettings() {
	p := s.child
	const id = pluginapi.SettingsWindow
	if p.windows[id].Root == nil {
		return
	}
	win := p.aux[id]
	if win == nil {
		win = p.window(id, false)
		p.aux[id] = win
		win.OnClosed(func() { delete(p.aux, id) })
	}
	win.Show()
	win.Focus()
}

func (s *pluginClientServices) CloseSettings() {
	if window := s.child.aux[pluginapi.SettingsWindow]; window != nil {
		window.Close()
	}
}

func (s *pluginClientServices) DesktopLyricsVisible() bool {
	return s.lyrics.Visible()
}

func (s *pluginClientServices) HideDesktopLyrics() {
	s.lyrics.Hide()
	s.Invalidate()
}

func (s *pluginClientServices) ToggleDesktopLyrics() {
	p := s.child
	if p.windows[pluginapi.DesktopLyricsWindow].Root == nil {
		return
	}
	if !s.lyrics.IsOpen() {
		s.lyrics.Open(p.windowOptions(pluginapi.DesktopLyricsWindow, true), func() bool { return p.quitting })
	}
	s.lyrics.Toggle()
	s.Invalidate()
}

func (s *pluginClientServices) DesktopLyricsLocked() bool { return s.lyrics.State().Locked }
func (s *pluginClientServices) DesktopLyricsState() pluginapi.DesktopLyricsState {
	state := s.lyrics.State()
	state.CustomColor = s.state.Desktop.DesktopLyricsColor
	state.FontFamily = s.state.Desktop.LyricsFontFamily
	return state
}
func (s *pluginClientServices) ToggleDesktopLyricsLock() {
	s.lyrics.ToggleLock()
	s.Invalidate()
}
func (s *pluginClientServices) SetDesktopLyricsStyle(translation bool, size float32, color int) {
	s.lyrics.SetStyle(translation, size, color)
}
func (s *pluginClientServices) SetMainWindowButtonsTheme(theme mygo.ThemeSource) {
	if s.child.main != nil {
		s.child.main.SetWindowButtonsTheme(theme)
	}
}

func (s *pluginClientServices) CurrentPlatform() pluginapi.ID { return s.child.entry.ID }

func (s *pluginClientServices) DecodedArtwork(raw string, size int, blur bool) pluginapi.DecodedArtwork {
	url := network.SizedArtworkURL(raw, size)
	if art := s.art.Get(url, url, blur); art != nil {
		return art
	}
	return nil
}
func (s *pluginClientServices) Artwork(raw string, size int) (*ui.Bitmap, bool) {
	if raw == "" {
		return nil, true
	}
	if art := s.DecodedArtwork(raw, size, false); art != nil {
		return art.Bitmap(), true
	}
	return nil, false
}

func (s *pluginClientServices) CopyText(text string) {
	_ = mygo.Clipboard.Write(transfer.TextData(text))
}
func (s *pluginClientServices) GlobalSearchHistory() []string {
	return s.state.Search
}
func (s *pluginClientServices) RememberGlobalSearch(q string) {
	s.send(bridge.PluginHostCall{Action: "history-add", Text: q})
}
func (s *pluginClientServices) ClearSearchHistory() { s.action("history-clear") }

func (s *pluginClientServices) DesktopSettings() pluginapi.DesktopSettings { return s.state.Desktop }
func (s *pluginClientServices) ApplyDesktopPreferences(p pluginapi.DesktopPreferences) {
	previous := s.state.Desktop.Preferences
	s.state.Desktop.Preferences = p
	if p.Appearance != previous.Appearance {
		ApplyAppearance(p.Appearance)
	}
	patch := bridge.WireDesktopPreferences(previous).Changes(bridge.WireDesktopPreferences(p))
	if patch.Fields != 0 {
		s.send(bridge.PluginHostCall{Action: protocol.HostPatchDesktopPreferences, DesktopPatch: patch})
	}
}
func (s *pluginClientServices) SetDesktopLyricsColor(color string) {
	if s.state.Desktop.DesktopLyricsColorAvailable {
		s.send(bridge.PluginHostCall{Action: protocol.HostSetDesktopLyricsColor, Text: color})
	}
}

func (s *pluginClientServices) SetStatusLyrics(enabled bool) {
	if s.state.Desktop.StatusLyricsAvailable {
		s.send(bridge.PluginHostCall{Action: protocol.HostSetStatusLyrics, Flag: enabled})
	}
}

func (s *pluginClientServices) SetTaskbarLyricsFontSize(size int) {
	if s.state.Desktop.TaskbarLyricsFontSize != 0 {
		s.send(bridge.PluginHostCall{Action: protocol.HostSetTaskbarLyricsFontSize, Index: size})
	}
}

func (s *pluginClientServices) SetTaskbarLyricsFontFamily(family string) {
	if s.state.Desktop.TaskbarLyricsFontSize != 0 {
		s.send(bridge.PluginHostCall{Action: protocol.HostSetTaskbarLyricsFontFamily, Text: family})
	}
}

func (s *pluginClientServices) SetLyricsFontFamily(family string) {
	if s.state.Desktop.LyricsFontAvailable {
		s.send(bridge.PluginHostCall{Action: protocol.HostSetLyricsFontFamily, Text: family})
	}
}

func (s *pluginClientServices) ClearAudioCache() { s.action("clear-cache") }
func (s *pluginClientServices) DecoderStatus() pluginapi.DecoderStatus {
	return s.state.Decoder
}
func (s *pluginClientServices) StartDecoderDownload() { s.action("decoder-download") }
func (s *pluginClientServices) CancelDecoderDownload() {
	s.send(bridge.PluginHostCall{Action: "decoder-cancel", TaskID: s.state.Decoder.DownloadTask.ID})
}
func (s *pluginClientServices) DeleteDecoder() { s.action("decoder-delete") }
func (s *pluginClientServices) SetUpdateChecks(on bool) {
	s.send(bridge.PluginHostCall{Action: "update-auto", Flag: on})
}
func (s *pluginClientServices) SetUpdateDownloads(on bool) {
	s.send(bridge.PluginHostCall{Action: "update-download", Flag: on})
}
func (s *pluginClientServices) CheckForUpdates() { s.action("update-check") }
func (s *pluginClientServices) Volume() float64  { return s.state.Volume }
func (s *pluginClientServices) SetVolume(n float64) {
	s.state.Volume = n
	s.send(bridge.PluginHostCall{Action: "volume", Number: n})
}
func (s *pluginClientServices) Presentation() pluginapi.Presentation { return s.state.Presentation }
func (s *pluginClientServices) SetLyricsPresentation(stage bool, style string, sparks bool) {
	view := s.state.Presentation
	view.LyricsStage, view.StageStyle, view.StageSparks = stage, style, sparks
	s.state.Presentation = view
	s.send(bridge.PluginHostCall{Action: "presentation", View: view})
}
func (s *pluginClientServices) Casting() pluginapi.CastingState { return s.state.Casting }
func (s *pluginClientServices) ScanCastingDevices()             { s.action("cast-scan") }
func (s *pluginClientServices) SwitchPlaybackOutput(id string) {
	s.send(bridge.PluginHostCall{Action: "cast-output", Text: id})
}
func (s *pluginClientServices) EqualizerAvailable() bool { return s.state.EQAvailable }
func (s *pluginClientServices) EqualizerSettings() eq.Settings {
	return s.state.EQ
}
func (s *pluginClientServices) ApplyEqualizer(p eq.Settings) {
	s.state.EQ = p
	s.send(bridge.PluginHostCall{Action: "equalizer", EQ: p})
}

func (s *pluginClientServices) Player() pluginapi.PlayerPort         { return s }
func (s *pluginClientServices) Playback() pluginapi.PlaybackControls { return s }
func (s *pluginClientServices) PlaybackLevels() ([pluginapi.SpectrumBands]float32, bool) {
	return s.Levels()
}
func (s *pluginClientServices) Current() *pluginapi.Track { return s.state.Current }
func (s *pluginClientServices) Playing() bool             { return s.state.Playing }
func (s *pluginClientServices) Loading() bool             { return s.state.Loading }
func (s *pluginClientServices) Ready() bool               { return s.state.Ready }
func (s *pluginClientServices) Err() string               { return s.state.Error }
func (s *pluginClientServices) Position() time.Duration {
	pos := s.state.Position
	if s.state.Playing && s.state.Ready && !s.state.Loading && !s.state.Casting.Sending {
		pos += time.Since(s.updated)
	}
	if s.state.Duration > 0 {
		pos = min(pos, s.state.Duration)
	}
	return pos
}
func (s *pluginClientServices) Duration() time.Duration { return s.state.Duration }
func (s *pluginClientServices) Downloaded() (float64, bool) {
	return s.state.Downloaded, s.state.DownloadKnown
}
func (s *pluginClientServices) TogglePlay() { s.action("resume-pause") }
func (s *pluginClientServices) Prev()       { s.action("previous") }
func (s *pluginClientServices) Next()       { s.action("next") }
func (s *pluginClientServices) Seek(pos time.Duration) {
	s.send(bridge.PluginHostCall{Action: "seek", Position: pos, HasRevision: true, Attempt: s.state.Attempt})
}
func (s *pluginClientServices) Shuffle() bool  { return s.state.Shuffle }
func (s *pluginClientServices) ToggleShuffle() { s.action("shuffle") }
func (s *pluginClientServices) SetShuffle(on bool) {
	s.send(bridge.PluginHostCall{Action: "set-shuffle", Flag: on})
}
func (s *pluginClientServices) Repeat() pluginapi.RepeatMode { return s.state.Repeat }
func (s *pluginClientServices) CycleRepeat()                 { s.action("repeat") }
func (s *pluginClientServices) Queue() []pluginapi.Track     { return s.state.Queue }
func (s *pluginClientServices) PlayOrder() ([]int, int)      { return s.state.Order, s.state.At }
func (s *pluginClientServices) Jump(i int) {
	s.send(bridge.PluginHostCall{Action: "jump", Index: i, HasRevision: true, QueueRevision: s.state.QueueRevision})
}
func (s *pluginClientServices) Lyrics() ([]lyrics.Line, bool) {
	return s.state.Lyrics, s.state.LyricsLoaded
}
func (s *pluginClientServices) Stream() *pluginapi.Stream { return s.state.Stream.Stream() }
func (s *pluginClientServices) Levels() ([pluginapi.SpectrumBands]float32, bool) {
	return s.state.Levels, s.state.LevelsLive
}
func (s *pluginClientServices) Radio() string { return s.state.Radio }
func (s *pluginClientServices) EndRadio()     { s.action("end-radio") }

func (s *pluginClientServices) PlayQueue(tracks []pluginapi.Track, at int) {
	clear(s.pagination)
	s.send(bridge.PluginHostCall{Action: "play", Tracks: tracks, Index: at})
}

func (s *pluginClientServices) AppendQueue(tracks []pluginapi.Track) {
	s.send(bridge.PluginHostCall{Action: "append", Tracks: tracks})
}

func (s *pluginClientServices) attachFeed(more pluginapi.QueueFeed) string {
	clear(s.pagination)
	if more == nil {
		return ""
	}
	s.nextPage++
	id := strconv.FormatUint(s.nextPage, 10)
	s.pagination[id] = &pluginFeed{next: more}
	return id
}

func (s *pluginClientServices) feed(id string) *pluginFeed {
	return s.pagination[id]
}

func (s *pluginClientServices) PlayList(tracks []pluginapi.Track, at int, more pluginapi.QueueFeed) {
	s.send(bridge.PluginHostCall{Action: "play", Tracks: tracks, Index: at, Feed: s.attachFeed(more)})
}

func (s *pluginClientServices) PlayRadio(name string, tracks []pluginapi.Track, more pluginapi.QueueFeed) {
	s.send(bridge.PluginHostCall{Action: "radio-play", Text: name, Tracks: tracks, Feed: s.attachFeed(more)})
}

var _ pluginapi.Services = (*pluginClientServices)(nil)

// AudioPreferencesChanged updates the active host without blocking the UI.
func (s *pluginClientServices) AudioPreferencesChanged(p pluginapi.AudioPreferences) {
	data, err := json.Marshal(p)
	if err == nil {
		s.send(bridge.PluginHostCall{Action: protocol.HostAudioPreferences, Settings: data})
	}
}
