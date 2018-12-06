package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nsf/termbox-go"
	"github.com/sachaos/todoist/lib"
	"github.com/urfave/cli"
)

// AppState represents one of the states that the UI can be in.
type AppState string

// List of possible AppState choices.
const (
	Init     AppState = "init"
	Ready    AppState = "ready"
	Run      AppState = "run"
	LoadData AppState = "loading"
	Stop     AppState = "stop"
)

// TuiApp manages the state for the Todoist TUI interface.
type TuiApp struct {
	Tasks            []todoist.Item
	PlaceholderTasks []string
	Projects         []todoist.Project
	CurrentProject   int
	Client           todoist.Client
	ErrorHandler     func(err error)
	ErrorMessage     string
	State            AppState
	events           chan termbox.Event
}

func newTuiApp(client *todoist.Client) (*TuiApp, error) {
	t := &TuiApp{}
	t.Client = *client
	t.State = Init
	t.PlaceholderTasks = []string{
		"item 1",
		"item 2",
		"item 3",
		"item 1",
		"item 2",
		"item 3",
		"item 1",
		"item 2",
		"item 3",
	}

	return t, nil
}

// Run orchestrates all of the moving pieces.
func (t *TuiApp) Run() {

	// Set up interrupt handler.
	stop := make(chan os.Signal, 2)
	go func() {
		<-stop
		t.State = Stop
		<-stop
		t.Stop() // second signal - exit directly.
	}()

	// Set up termbox.
	err := termbox.Init()
	if err != nil {
		panic(err)
	}
	termbox.SetInputMode(termbox.InputEsc)

	// Load the initial data.
	t.State = LoadData
	t.Draw()
	t.Client.Sync(context.Background())
	t.Projects = t.Client.Store.Projects
	t.CurrentProject = 0

	// Draw the initial UI and set the state to Ready.
	t.State = Ready
	t.Draw()

	// Queue up termbox events for processing.
	t.events = make(chan termbox.Event)
	go func() {
		for {
			t.events <- termbox.PollEvent()
		}
	}()

	// The main loop should run at 60hz.
	for range time.Tick(time.Duration(1000/60) * time.Millisecond) {
		// If the last loop iteration put the app into a stop state, break out of the loop
		if t.State == Stop {
			break
		}

		t.ProcessInput()
		t.Draw()
	}

	// Complete any shutdown tasks.
	t.Stop()
}

// ProcessInput will process an event from the t.events channel.
func (t *TuiApp) ProcessInput() {
	var curEvent termbox.Event

	select {
	case e, ok := <-t.events:
		curEvent = e
		if !ok {
			// Channel was closed and we need to stop the application.
			t.State = Stop
			return
		}
	}

	// If we've gotten to this point, we have an event that's ready to process.
	switch curEvent.Type {
	case termbox.EventKey:
		switch curEvent.Key {

		case termbox.KeyEsc:
			t.State = Stop

		case termbox.KeyCtrlC:
			t.State = Stop

		case termbox.KeyPgdn:
			t.CurrentProject++
			if t.CurrentProject > len(t.Projects)-1 {
				t.CurrentProject = 0
			}

		case termbox.KeyPgup:
			t.CurrentProject--
			if t.CurrentProject < 0 {
				t.CurrentProject = len(t.Projects) - 1
			}

		}

	// If there's an error or an interrupt, stop the application.
	case termbox.EventInterrupt:
		fallthrough
	case termbox.EventError:
		t.ErrorMessage = curEvent.Err.Error()
		t.State = Stop
	}
}

// Stop handles all app shutdown tasks.
func (t *TuiApp) Stop() {
	termbox.Close()

	if t.ErrorMessage != "" {
		fmt.Println(t.ErrorMessage)
	}
}

// Draw clears the screen, redraws everything, then flushes the result.
func (t *TuiApp) Draw() {
	w, h := termbox.Size()
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	defer termbox.Flush()

	// Draw a list of projects.
	for x := 0; x < w; x++ {
		termbox.SetCell(x, 0, ' ', termbox.ColorDefault, termbox.ColorWhite)
	}
	t.drawString(0, 0, "Projects", true)

	// If we're loading, don't draw the rest of the UI.
	if t.State == LoadData {
		t.drawLoadingOverlay()
		return
	}

	// Draw a list of projects.
	longestProjectName := 0
	for key, project := range t.Projects {
		if len(project.Name) > longestProjectName {
			longestProjectName = len(project.Name)
		}

		if key == t.CurrentProject {
			t.drawString(0, 2+key, "> "+project.Name, false)
		} else {
			t.drawString(2, 2+key, project.Name, false)
		}
	}

	// Draw vertical divider.
	for y := 0; y < h; y++ {
		termbox.SetCell(longestProjectName+4, y, ' ', termbox.ColorDefault, termbox.ColorWhite)
	}
}

// Draw loading screen.
func (t *TuiApp) drawLoadingOverlay() {
	w, _ := termbox.Size()
	t.drawString(w-10, 0, "Loading...", true)
}

// drawString puts text on the screen starting and the specified cell (x, y).
func (t *TuiApp) drawString(x, y int, str string, invert bool) {
	for pos, char := range str {
		if invert {
			termbox.SetCell(x+pos, y, char, termbox.ColorBlack, termbox.ColorWhite)
		} else {
			termbox.SetCell(x+pos, y, char, termbox.ColorDefault, termbox.ColorDefault)
		}
	}
}

// TUI is the CLI handler that sets up the TUI app and runs it.
func TUI(c *cli.Context) error {
	client := GetClient(c)

	app, err := newTuiApp(client)
	if err != nil {
		return CommandFailed
	}

	app.Run()

	return nil
}
