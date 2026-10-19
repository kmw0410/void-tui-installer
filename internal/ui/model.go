package ui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"void-installer/internal/installer"
	"void-installer/internal/validation"
)

type screen int

const (
	welcome screen = iota
	mirror
	disks
	account
	timezone
	network
	desktop
	utilities
	review
	installing
	result
)

var stageNames = installer.Stages

type environmentMsg struct {
	disks      []installer.Disk
	interfaces []string
	gpus       []installer.GPU
	err        error
}
type preparationMsg struct {
	stage, status string
	updates       <-chan tea.Msg
}
type startedMsg struct {
	session *installer.Session
	err     error
}
type repositoryMsg struct {
	root string
	err  error
}
type packagesMsg struct {
	root     string
	packages []installer.Package
	err      error
}
type eventMsg struct {
	event installer.Event
	open  bool
}
type doneMsg struct{ err error }
type pulseMsg time.Time

type model struct {
	ctx                               context.Context
	cancel                            context.CancelFunc
	jobs                              sync.WaitGroup
	lifecycle                         sync.Mutex
	stopped                           bool
	version                           string
	screen                            screen
	width, height                     int
	ready, checking                   bool
	err                               string
	fieldErrors                       map[int]string
	preparation                       map[string]string
	continueFocused                   bool
	diskInspection                    bool
	disks                             []installer.Disk
	interfaces                        []string
	gpus                              []installer.GPU
	selectedDesktop, selectedUtility  int
	desktopFocus                      int
	selectedMirror                    int
	selectedTimezone                  int
	repositoryReady                   bool
	packages                          []installer.Package
	packagesRoot                      string
	packagesLoading                   bool
	packagesError                     string
	packageFilter                     textinput.Model
	selectedOnly                      bool
	packageFocus                      int
	settingsMode                      string
	settingsPath                      textinput.Model
	settingsBusy                      bool
	settingsNotice                    string
	reviewChecking                    bool
	reviewCheckID                     int
	reviewReport                      installer.CheckReport
	reviewCheckError                  string
	reviewCheckedConfig               string
	reviewCancel                      context.CancelFunc
	reviewOffset                      int
	reviewFocus                       int
	selectedDisk, selectedInterface   int
	config                            installer.Config
	fields                            []textinput.Model
	focus                             int
	static                            bool
	session                           *installer.Session
	starting, abortPrompt, cancelling bool
	stages                            map[string]string
	current                           string
	logs                              []string
	showLogs                          bool
	logOffset                         int
	pulse                             int
	success                           bool
}

func New() tea.Model { return NewWithContext(context.Background()) }

func (m *model) SetMirror(value string) error {
	root, err := installer.NormalizeMirror(value)
	if err != nil {
		return err
	}
	m.selectedMirror = len(installer.Mirrors)
	m.fields[9].SetValue("")
	for i, item := range installer.Mirrors {
		if item.URL == root {
			m.selectedMirror = i
			break
		}
	}
	if m.selectedMirror == len(installer.Mirrors) {
		m.fields[9].SetValue(root)
		m.fields[9].CursorEnd()
	}
	m.repositoryReady = false
	return nil
}

func NewWithContext(parent context.Context) *model {
	return NewWithVersion(parent, "dev")
}

func NewWithVersion(parent context.Context, version string) *model {
	ctx, cancel := context.WithCancel(parent)
	m := &model{ctx: ctx, cancel: cancel, version: version, screen: welcome, checking: true, config: installer.Config{Version: 1, Network: "dhcp", Desktop: "none", Graphics: "default", Timezone: "UTC", Utilities: []string{}}, stages: map[string]string{}}
	m.selectedTimezone = slices.Index(installer.Timezones(), "UTC")
	for i := 0; i < 11; i++ {
		input := textinput.New()
		input.Prompt = "> "
		input.CharLimit = 128
		input.SetWidth(40)
		if i == 1 || i == 2 {
			input.EchoMode = textinput.EchoPassword
			input.EchoCharacter = '*'
		}
		m.fields = append(m.fields, input)
	}
	m.fields[3].SetValue("void")
	m.fields[9].CharLimit = 512
	m.packageFilter = textinput.New()
	m.packageFilter.Prompt = "> "
	m.packageFilter.CharLimit = 128
	m.packageFilter.SetWidth(40)
	m.settingsPath = textinput.New()
	m.settingsPath.Prompt = "> "
	m.settingsPath.CharLimit = 4096
	m.settingsPath.SetWidth(50)
	return m
}

func checkEnvironment(ctx context.Context, report func(string, string)) tea.Msg {
	if err := installer.PrepareLiveProgressContext(ctx, report); err != nil {
		return environmentMsg{err: err}
	}
	report("discovery", "running")
	d, err := installer.DiscoverContext(ctx)
	if err != nil {
		return environmentMsg{err: err}
	}
	interfaces, err := installer.Interfaces()
	if err != nil {
		return environmentMsg{err: err}
	}
	gpus, err := installer.NVIDIAGPUs()
	if err == nil {
		report("discovery", "completed")
	}
	return environmentMsg{disks: d, interfaces: interfaces, gpus: gpus, err: err}
}
func pulse() tea.Cmd {
	return tea.Tick(time.Second/3, func(t time.Time) tea.Msg { return pulseMsg(t) })
}
func (m *model) Init() tea.Cmd { return tea.Batch(m.checkCommand(), pulse()) }

func (m *model) checkCommand() tea.Cmd {
	return func() tea.Msg {
		if !m.beginJob() {
			return environmentMsg{err: context.Canceled}
		}
		updates := make(chan tea.Msg, 32)
		go func() {
			defer m.jobs.Done()
			defer close(updates)
			report := func(stage, status string) {
				select {
				case updates <- preparationMsg{stage, status, updates}:
				case <-m.ctx.Done():
				}
			}
			result := checkEnvironment(m.ctx, report)
			select {
			case updates <- result:
			case <-m.ctx.Done():
			}
		}()
		return <-updates
	}
}
func (m *model) repositoryCommand(root string) tea.Cmd {
	return func() tea.Msg {
		if !m.beginJob() {
			return repositoryMsg{root: root, err: context.Canceled}
		}
		defer m.jobs.Done()
		return repositoryMsg{root: root, err: installer.CheckMirrorContext(m.ctx, root)}
	}
}

func (m *model) beginJob() bool {
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	if m.stopped {
		return false
	}
	m.jobs.Add(1)
	return true
}

func (m *model) Shutdown() {
	m.lifecycle.Lock()
	m.stopped = true
	m.cancel()
	m.lifecycle.Unlock()
	m.jobs.Wait()
}
func nextEvent(s *installer.Session) tea.Cmd {
	return func() tea.Msg { e, ok := <-s.Events; return eventMsg{e, ok} }
}
func awaitDone(s *installer.Session) tea.Cmd { return func() tea.Msg { return doneMsg{<-s.Done} } }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case reviewCheckedMsg:
		if msg.id != m.reviewCheckID || msg.key != configKey(m.config) {
			return m, nil
		}
		m.reviewChecking = false
		m.reviewReport = msg.report
		m.reviewCheckError = ""
		if msg.err != nil {
			m.reviewCheckError = msg.err.Error()
		} else {
			m.reviewCheckedConfig = msg.key
		}
		return m, nil
	case settingsMsg:
		m.settingsBusy = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.settingsMode = ""
		m.err = ""
		if msg.mode == "load" {
			return m, m.applyLoadedSettings(msg)
		}
		m.settingsNotice = "Settings saved: " + msg.path
		return m, m.resumeSettingsFocus()
	case tea.PasteMsg:
		var cmd tea.Cmd
		if m.settingsMode != "" && !m.settingsBusy {
			m.settingsPath, cmd = m.settingsPath.Update(msg)
		} else if m.screen == utilities && m.packageFocus == 0 {
			m.packageFilter, cmd = m.packageFilter.Update(msg)
			m.selectedUtility = 0
		}
		return m, cmd
	case preparationMsg:
		if m.preparation == nil {
			m.preparation = map[string]string{}
		}
		m.preparation[msg.stage] = msg.status
		return m, func() tea.Msg { return <-msg.updates }
	case repositoryMsg:
		m.checking = false
		m.fieldErrors = nil
		if msg.err != nil {
			m.repositoryReady = false
			m.err = msg.err.Error()
			return m, nil
		}
		if m.config.Mirror != msg.root {
			m.packages = nil
			m.packagesRoot = ""
			m.packagesLoading = false
			m.config.Utilities = nil
		}
		m.config.Mirror = msg.root
		m.diskInspection = false
		m.repositoryReady = true
		m.err = ""
		m.screen = disks
		m.blurAll()
		return m, nil
	case packagesMsg:
		if msg.root != m.config.Mirror {
			return m, nil
		}
		m.packagesLoading = false
		if msg.err != nil {
			m.packagesError = msg.err.Error()
			return m, nil
		}
		m.packagesError = ""
		m.packages, m.packagesRoot = msg.packages, msg.root
		m.selectedUtility = 0
		m.err = ""
		available := make(map[string]bool, len(msg.packages))
		for _, p := range msg.packages {
			available[p.Name] = true
		}
		previous := len(m.config.Utilities)
		m.config.Utilities = slices.DeleteFunc(m.config.Utilities, func(name string) bool { return !available[name] })
		if len(m.config.Utilities) != previous {
			m.err = "Unavailable packages were removed from the selection."
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		for i := range m.fields {
			m.fields[i].SetWidth(max(15, min(40, m.width-46)))
		}
		m.packageFilter.SetWidth(max(15, min(40, m.width-46)))
		m.settingsPath.SetWidth(max(15, min(60, m.width-16)))
		return m, nil
	case environmentMsg:
		m.checking = false
		m.ready = false
		m.disks, m.interfaces = msg.disks, msg.interfaces
		m.gpus = msg.gpus
		if msg.err != nil {
			m.err = msg.err.Error()
			for stage, status := range m.preparation {
				if status == "running" {
					m.preparation[stage] = "failed"
				}
			}
			return m, nil
		}
		m.selectedDisk = -1
		for i, d := range m.disks {
			if d.Safe {
				m.selectedDisk = i
				break
			}
		}
		if m.selectedDisk < 0 {
			m.err = "No safe installation disk is available."
			if len(m.disks) == 0 {
				m.err += "\nNo whole disks were detected."
			}
			for i, d := range m.disks {
				if i >= 4 {
					m.err += "\nAdditional blocked disks were omitted."
					break
				}
				m.err += "\n" + d.Path + ": " + d.Reason
			}
			return m, nil
		}
		if len(m.interfaces) == 0 {
			m.err = "No wired network interface is available."
			return m, nil
		}
		m.ready = true
		m.fieldErrors = nil
		m.err = ""
		return m, nil
	case pulseMsg:
		m.pulse++
		return m, pulse()
	case startedMsg:
		m.starting = false
		m.clearSecrets()
		if msg.err != nil {
			m.screen = result
			m.err = msg.err.Error()
			return m, nil
		}
		m.session = msg.session
		if m.cancelling {
			m.session.Cancel()
		}
		return m, tea.Batch(nextEvent(m.session), awaitDone(m.session))
	case eventMsg:
		if !msg.open {
			return m, nil
		}
		e := msg.event
		if e.Type == "stage" {
			m.stages[e.Name] = e.Status
			m.current = e.Name
		}
		if e.Message != "" {
			m.appendLog(e.Message)
		}
		return m, nextEvent(m.session)
	case doneMsg:
		m.screen = result
		m.abortPrompt = false
		m.success = msg.err == nil && !m.cancelling
		if msg.err != nil {
			m.err = msg.err.Error()
		} else if m.cancelling {
			m.err = "Installation was cancelled. The target disk may be incomplete."
		}
		return m, nil
	case tea.KeyPressMsg:
		key := msg.String()
		if m.settingsMode != "" {
			return m, m.updateSettingsDialog(msg)
		}
		m.settingsNotice = ""
		if m.screen == installing {
			if key == "f2" {
				m.showLogs = !m.showLogs
				return m, nil
			}
			if m.abortPrompt {
				switch key {
				case "y", "Y":
					m.abortPrompt = false
					m.cancelling = true
					if m.session != nil {
						m.session.Cancel()
					}
				case "n", "N", "esc":
					m.abortPrompt = false
				}
				return m, nil
			}
			if key == "ctrl+c" {
				if !m.cancelling {
					m.abortPrompt = true
				}
				return m, nil
			}
			if m.showLogs {
				m.scrollLogs(key)
			}
			return m, nil
		}
		if key == "ctrl+c" {
			m.clearSecrets()
			return m, tea.Quit
		}
		if m.width < 80 || m.height < 24 {
			if key == "q" {
				return m, tea.Quit
			}
			return m, nil
		}
		if m.screen == result {
			if key == "f2" {
				m.showLogs = !m.showLogs
			} else if key == "q" || key == "enter" || key == "esc" {
				return m, tea.Quit
			} else if m.showLogs {
				m.scrollLogs(key)
			}
			return m, nil
		}
		if m.screen == mirror && m.checking {
			return m, nil
		}
		if (key == "f5" || key == "f9") && m.screen <= review {
			return m, m.openSettingsDialog(key == "f5")
		}
		if key == "esc" {
			if m.screen == review && m.reviewCancel != nil {
				m.reviewCancel()
				m.reviewCheckID++
				m.reviewChecking = false
			}
			m.err = ""
			m.fieldErrors = nil
			m.continueFocused = false
			if m.screen == disks && m.diskInspection {
				m.screen = welcome
				m.diskInspection = false
				return m, nil
			}
			if m.screen > welcome {
				m.screen--
				if m.screen == mirror && m.selectedMirror == len(installer.Mirrors) {
					return m, m.setFocus(9)
				}
				if m.screen == account {
					return m, m.setFocus(0)
				}
				if m.screen == timezone {
					return m, m.setFocus(10)
				}
				if m.screen == utilities {
					return m, m.focusPackages(0)
				}
				if m.screen == network && m.static {
					return m, m.setFocus(4)
				}
				m.blurAll()
			}
			return m, nil
		}
		if m.screen == utilities && key == "ctrl+r" && !m.packagesLoading {
			m.continueFocused = false
			m.packagesRoot = ""
			return m, m.openPackages()
		}
		if m.screen >= mirror && m.screen < utilities && m.screen != account && !(m.screen == network && m.static) {
			if m.continueFocused {
				if key == "tab" || key == "shift+tab" || key == "up" || key == "down" {
					m.continueFocused = false
					if m.screen == mirror && m.selectedMirror == len(installer.Mirrors) {
						return m, m.setFocus(9)
					}
					if m.screen == timezone {
						return m, m.setFocus(10)
					}
					if m.screen == utilities {
						return m, m.setFocus(14)
					}
					return m, nil
				}
				if key != "enter" {
					return m, nil
				}
				m.continueFocused = false
			} else if key == "tab" {
				if m.screen == disks && (m.selectedDisk < 0 || !m.disks[m.selectedDisk].Safe) {
					return m, nil
				}
				if m.screen == desktop && m.desktopFocus < 3 {
					m.selectedDesktop = m.desktopFocus
				}
				m.continueFocused = true
				m.blurAll()
				return m, nil
			}
		}
		switch m.screen {
		case welcome:
			if key == "q" {
				return m, tea.Quit
			}
			if key == "r" && !m.checking {
				m.ready = false
				m.checking = true
				m.err = ""
				m.preparation = map[string]string{}
				return m, m.checkCommand()
			}
			if key == "enter" && m.ready {
				m.screen = mirror
			}
			if key == "d" && !m.checking && !m.ready && len(m.disks) > 0 {
				m.diskInspection = true
				m.selectedDisk = 0
				m.screen = disks
				m.err = ""
			}
		case mirror:
			switch key {
			case "up", "shift+tab":
				m.selectedMirror = max(0, m.selectedMirror-1)
				m.blurAll()
				return m, nil
			case "down", "tab":
				m.selectedMirror = min(len(installer.Mirrors), m.selectedMirror+1)
				if m.selectedMirror == len(installer.Mirrors) {
					return m, m.setFocus(9)
				}
				m.blurAll()
				return m, nil
			case "enter":
				root := m.fields[9].Value()
				if m.selectedMirror < len(installer.Mirrors) {
					root = installer.Mirrors[m.selectedMirror].URL
				} else if root == "" {
					m.err = "Enter the custom mirror root."
					m.fieldErrors = map[int]string{9: m.err}
					m.setFocus(9)
					return m, nil
				}
				normalized, err := installer.NormalizeMirror(root)
				if err != nil {
					m.err = err.Error()
					if m.selectedMirror == len(installer.Mirrors) {
						m.fieldErrors = map[int]string{9: m.err}
						m.setFocus(9)
					}
					return m, nil
				}
				if m.repositoryReady && normalized == m.config.Mirror {
					m.screen = disks
					m.blurAll()
					return m, nil
				}
				m.repositoryReady = false
				m.checking = true
				m.err = ""
				m.fieldErrors = nil
				return m, m.repositoryCommand(normalized)
			}
			if m.selectedMirror == len(installer.Mirrors) {
				return m, m.updateField(msg, 9)
			}
		case disks:
			switch key {
			case "up", "shift+tab":
				m.selectedDisk = max(0, m.selectedDisk-1)
			case "down", "tab":
				m.selectedDisk = min(len(m.disks)-1, m.selectedDisk+1)
			case "enter":
				if !m.diskInspection && m.selectedDisk >= 0 && m.selectedDisk < len(m.disks) && m.disks[m.selectedDisk].Safe {
					d := m.disks[m.selectedDisk]
					m.config.Disk = d.Path
					m.config.Identity = d.Identity
					m.screen = account
					return m, m.setFocus(0)
				}
			}
		case account:
			if key == "tab" || key == "down" {
				if m.focus == 13 {
					return m, m.setFocus(0)
				}
				if m.focus == 3 {
					return m, m.setFocus(13)
				}
				return m, m.setFocus(m.focus + 1)
			}
			if key == "enter" {
				if m.focus < 4 {
					if m.focus == 3 {
						return m, m.setFocus(13)
					}
					return m, m.setFocus(m.focus + 1)
				}
				if err := m.validateAccount(); err != nil {
					m.err = err.Error()
					return m, nil
				}
				m.err = ""
				m.screen = timezone
				return m, m.setFocus(10)
			}
			if key == "shift+tab" || key == "up" {
				if m.focus == 0 {
					return m, m.setFocus(13)
				}
				if m.focus == 13 {
					return m, m.setFocus(3)
				}
				return m, m.setFocus(m.focus - 1)
			}
			if m.focus < 4 {
				return m, m.updateField(msg, m.focus)
			}
			return m, nil
		case timezone:
			zones := m.filteredTimezones()
			switch key {
			case "up", "shift+tab":
				m.selectedTimezone = max(0, m.selectedTimezone-1)
			case "down", "tab":
				m.selectedTimezone = max(0, min(len(zones)-1, m.selectedTimezone+1))
			case "pgup":
				m.selectedTimezone = max(0, m.selectedTimezone-10)
			case "pgdown":
				m.selectedTimezone = max(0, min(len(zones)-1, m.selectedTimezone+10))
			case "enter":
				if len(zones) == 0 {
					m.err = "No matching timezone. Change the filter."
					m.fieldErrors = map[int]string{10: m.err}
					return m, nil
				}
				m.config.Timezone = zones[m.selectedTimezone]
				m.fieldErrors = nil
				m.screen = network
				m.err = ""
				m.blurAll()
				if m.static {
					return m, m.setFocus(4)
				}
			default:
				before := m.fields[10].Value()
				cmd := m.updateField(msg, 10)
				if before != m.fields[10].Value() {
					m.selectedTimezone = 0
				}
				return m, cmd
			}
		case network:
			if !m.static {
				if key == "left" || key == "right" || key == "space" {
					m.static = true
					return m, m.setFocus(4)
				}
				if key == "enter" {
					m.config.Network = "dhcp"
					m.config.Interface, m.config.Address, m.config.Gateway, m.config.DNS = "", "", "", ""
					m.screen = desktop
					m.err = ""
					m.blurAll()
					return m, nil
				}
			} else {
				if m.focus == 11 && (key == "space" || key == "left" || key == "right") {
					m.static = false
					m.fieldErrors = nil
					m.err = ""
					m.blurAll()
					return m, nil
				}
				if m.focus == 12 && (key == "space" || key == "left" || key == "right") {
					m.selectedInterface = (m.selectedInterface + 1) % len(m.interfaces)
					delete(m.fieldErrors, 12)
					return m, nil
				}
				if key == "tab" || key == "down" || key == "shift+tab" || key == "up" {
					order := []int{11, 12, 4, 5, 6, 7}
					i := slices.Index(order, m.focus)
					delta := 1
					if key == "shift+tab" || key == "up" {
						delta = -1
					}
					return m, m.setFocus(order[(i+delta+len(order))%len(order)])
				}
				if key == "enter" {
					if m.focus == 11 {
						return m, m.setFocus(12)
					}
					if m.focus == 12 {
						return m, m.setFocus(4)
					}
					if m.focus < 7 {
						return m, m.setFocus(max(4, m.focus+1))
					}
					m.config.Network = "static"
					m.config.Interface = m.interfaces[m.selectedInterface]
					m.config.Address = m.fields[4].Value()
					m.config.Gateway = m.fields[5].Value()
					m.config.DNS = m.fields[6].Value()
					// The final network field is a continue control, with no input value.
					if err := m.reviewConfig(); err != nil {
						m.err = err.Error()
						field := 4
						if strings.Contains(m.err, "Gateway") {
							field = 5
						}
						if strings.Contains(m.err, "DNS") {
							field = 6
						}
						if strings.Contains(m.err, "interface") {
							field = 12
						}
						m.fieldErrors = map[int]string{field: m.err}
						m.setFocus(field)
						return m, nil
					}
					m.screen = desktop
					m.err = ""
					m.blurAll()
					return m, nil
				}
				if m.focus >= 4 && m.focus <= 6 {
					return m, m.updateField(msg, m.focus)
				}
			}
		case desktop:
			switch key {
			case "up", "shift+tab":
				m.desktopFocus = max(0, m.desktopFocus-1)
			case "down", "tab":
				last := 2
				if len(m.gpus) > 0 {
					last = 3
				}
				m.desktopFocus = min(last, m.desktopFocus+1)
			case "space":
				if m.desktopFocus < 3 {
					m.selectedDesktop = m.desktopFocus
				}
				if m.desktopFocus == 3 && len(m.gpus) > 0 {
					if m.config.Graphics == "nvidia" {
						m.config.Graphics = "default"
					} else {
						m.config.Graphics = "nvidia"
					}
				}
			case "enter":
				if m.desktopFocus < 3 {
					m.selectedDesktop = m.desktopFocus
				}
				m.config.Desktop = []string{"none", "kde", "niri"}[m.selectedDesktop]
				m.screen = utilities
				m.err = ""
				return m, m.openPackages()
			}
		case utilities:
			if key == "tab" || key == "shift+tab" {
				delta := 1
				if key == "shift+tab" {
					delta = -1
				}
				return m, m.focusPackages((m.packageFocus + delta + 4) % 4)
			}
			if m.packageFocus == 1 && (key == "space" || key == "enter") {
				m.selectedOnly = !m.selectedOnly
				m.selectedUtility = 0
				return m, nil
			}
			if m.packageFocus == 1 && (key == "up" || key == "down") {
				if key == "up" {
					return m, m.focusPackages(0)
				}
				return m, m.focusPackages(2)
			}
			if m.packageFocus == 3 && (key == "up" || key == "down") {
				if key == "up" {
					return m, m.focusPackages(2)
				}
				return m, m.focusPackages(0)
			}
			if m.packageFocus == 1 || (m.packageFocus == 3 && key != "enter") {
				return m, nil
			}
			packages := m.filteredPackages()
			switch key {
			case "up", "shift+tab":
				m.selectedUtility = max(0, m.selectedUtility-1)
			case "down", "tab":
				m.selectedUtility = max(0, min(len(packages)-1, m.selectedUtility+1))
			case "pgdown":
				m.selectedUtility = max(0, min(len(packages)-1, m.selectedUtility+10))
			case "pgup":
				m.selectedUtility = max(0, m.selectedUtility-10)
			case "space":
				if len(packages) == 0 || m.packagesLoading || m.packagesError != "" {
					return m, nil
				}
				p := packages[m.selectedUtility].Name
				if i := slices.Index(m.config.Utilities, p); i >= 0 {
					m.config.Utilities = slices.Delete(m.config.Utilities, i, i+1)
				} else {
					if len(m.config.Utilities) >= installer.MaxExtraPackages {
						m.err = "Select at most 256 additional packages."
						return m, nil
					}
					m.config.Utilities = append(m.config.Utilities, p)
				}
				m.err = ""
				m.selectedUtility = max(0, min(m.selectedUtility, len(m.filteredPackages())-1))
			case "enter":
				if m.packagesLoading || m.packagesError != "" || m.packagesRoot != m.config.Mirror {
					m.err = "Wait for the package list or press Ctrl+R to retry."
					return m, nil
				}
				if err := installer.ValidateRepositorySelection(m.config.Utilities, m.packages); err != nil {
					m.err = err.Error()
					return m, nil
				}
				return m, m.toReview()
			default:
				if m.packageFocus != 0 {
					return m, nil
				}
				before := m.packageFilter.Value()
				var cmd tea.Cmd
				m.packageFilter, cmd = m.packageFilter.Update(msg)
				if before != m.packageFilter.Value() {
					m.selectedUtility = 0
				}
				return m, cmd
			}
		case review:
			if key == "ctrl+r" && !m.reviewChecking {
				return m, m.checkReview()
			}
			if key == "pgdown" {
				m.reviewOffset += max(1, m.height-15)
				return m, nil
			}
			if key == "pgup" {
				m.reviewOffset = max(0, m.reviewOffset-max(1, m.height-15))
				return m, nil
			}
			if key == "tab" || key == "shift+tab" || key == "up" || key == "down" {
				delta := 1
				if key == "shift+tab" || key == "up" {
					delta = -1
				}
				m.reviewFocus = (m.reviewFocus + delta + 3) % 3
				m.blurAll()
				if m.reviewFocus == 0 {
					return m, m.setFocus(8)
				}
				return m, nil
			}
			if key == "enter" {
				if m.reviewFocus == 1 {
					if m.reviewCancel != nil {
						m.reviewCancel()
					}
					m.reviewCheckID++
					m.reviewChecking = false
					m.screen = utilities
					m.err = ""
					m.blurAll()
					return m, m.focusPackages(0)
				}
				if m.reviewFocus == 0 {
					m.reviewFocus = 2
					m.blurAll()
					return m, nil
				}
				if !m.erasureConfirmed() {
					return m, nil
				}
				m.config.Confirmation = m.fields[8].Value()
				if err := m.config.Validate(); err != nil {
					m.err = err.Error()
					return m, nil
				}
				if err := validatePassword(m.fields[1].Value(), m.fields[2].Value()); err != nil {
					m.err = err.Error()
					return m, nil
				}
				c, p := m.config, m.fields[1].Value()
				m.screen = installing
				m.starting = true
				m.err = ""
				m.blurAll()
				return m, func() tea.Msg {
					if !m.beginJob() {
						return startedMsg{err: context.Canceled}
					}
					s, err := installer.Start(m.ctx, c, p)
					if err != nil {
						m.jobs.Done()
					} else {
						go func() { s.Wait(); m.jobs.Done() }()
					}
					return startedMsg{s, err}
				}
			}
			if m.reviewFocus == 0 {
				return m, m.updateField(msg, 8)
			}
			return m, nil
		}
	}
	return m, nil
}
func (m *model) scrollLogs(key string) {
	switch key {
	case "up":
		m.logOffset = min(max(0, len(m.logs)-1), m.logOffset+1)
	case "pgup":
		m.logOffset = min(max(0, len(m.logs)-1), m.logOffset+10)
	case "down":
		m.logOffset = max(0, m.logOffset-1)
	case "pgdown":
		m.logOffset = max(0, m.logOffset-10)
	case "end":
		m.logOffset = 0
	case "home":
		m.logOffset = max(0, len(m.logs)-1)
	}
}
func (m *model) filteredTimezones() []string {
	filter := strings.ToLower(strings.TrimSpace(m.fields[10].Value()))
	zones := installer.Timezones()
	return slices.DeleteFunc(zones, func(zone string) bool {
		return !strings.Contains(strings.ToLower(zone), filter)
	})
}
func (m *model) appendLog(s string) {
	for _, line := range strings.Split(s, "\n") {
		m.logs = append(m.logs, line)
	}
	if len(m.logs) > 2000 {
		m.logs = m.logs[len(m.logs)-2000:]
	}
}
func (m *model) blurAll() {
	m.packageFilter.Blur()
	m.settingsPath.Blur()
	for i := range m.fields {
		m.fields[i].Blur()
	}
}
func (m *model) setFocus(i int) tea.Cmd {
	m.blurAll()
	m.focus = i
	if i == 14 {
		return m.packageFilter.Focus()
	}
	if i >= 0 && i < len(m.fields) {
		return m.fields[i].Focus()
	}
	return nil
}
func (m *model) updateField(msg tea.Msg, i int) tea.Cmd {
	var cmd tea.Cmd
	before := m.fields[i].Value()
	m.fields[i], cmd = m.fields[i].Update(msg)
	if before != m.fields[i].Value() {
		delete(m.fieldErrors, i)
	}
	m.err = ""
	return cmd
}
func (m *model) clearSecrets() { m.fields[1].SetValue(""); m.fields[2].SetValue("") }
func validatePassword(p, confirm string) error {
	if p != confirm {
		return errors.New("Passwords do not match.")
	}
	if err := validation.Password(p); err != nil {
		return err
	}
	return nil
}
func (m *model) validateAccount() error {
	m.fieldErrors = map[int]string{}
	if err := validation.Account(m.fields[0].Value(), "void"); err != nil {
		m.fieldErrors[0] = err.Error()
	}
	if err := validation.Password(m.fields[1].Value()); err != nil {
		m.fieldErrors[1] = err.Error()
	}
	if m.fields[1].Value() != m.fields[2].Value() {
		m.fieldErrors[2] = "Passwords do not match."
	}
	if err := validation.Account("voiduser", m.fields[3].Value()); err != nil {
		m.fieldErrors[3] = err.Error()
	}
	for i := 0; i < 4; i++ {
		if message := m.fieldErrors[i]; message != "" {
			m.setFocus(i)
			return errors.New(message)
		}
	}
	m.config.Username = m.fields[0].Value()
	m.config.Hostname = m.fields[3].Value()
	return validation.Account(m.config.Username, m.config.Hostname)
}
func (m *model) reviewConfig() error {
	c := m.config
	c.Confirmation = "ERASE " + c.Disk
	return c.Validate()
}
func (m *model) toReview() tea.Cmd {
	m.screen = review
	m.err = ""
	m.reviewOffset = 0
	m.reviewFocus = 0
	m.fields[8].SetValue("")
	m.setFocus(8)
	return m.checkReview()
}
func (m *model) networkSummary() string {
	if m.config.Network == "dhcp" {
		return "DHCP (wired interfaces)"
	}
	return fmt.Sprintf("Static: %s, %s\nGateway: %s   DNS: %s", m.config.Interface, m.config.Address, m.config.Gateway, m.config.DNS)
}

func (m *model) erasureConfirmed() bool {
	return m.fields[8].Value() == "ERASE "+m.config.Disk && m.reviewChecksPassed()
}
